package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	debugoverlay "github.com/marianogappa/scmapanalyzer/internal/debug/overlay"
	"github.com/marianogappa/scmapanalyzer/internal/matcher"
	"github.com/marianogappa/scmapanalyzer/internal/model"
	"github.com/marianogappa/scmapanalyzer/internal/replay"
	"github.com/marianogappa/scmapanalyzer/internal/tiletags"
	"github.com/marianogappa/scmapanalyzer/replaymap"
)

type imageCandidate struct {
	Path string
	Stem string
}

type replayCandidate struct {
	Path string
	Meta *model.MapMetadata
}

type selectedRun struct {
	ReplayPath string
	ImagePath  string
	MapName    string
	Score      float64
}

func main() {
	var (
		replaysDir   string
		mapImagesDir string
		outputDir    string
		onlyRunMap   string
	)
	flag.StringVar(&replaysDir, "replays-dir", "replays", "Directory to scan recursively for .rep files")
	flag.StringVar(&mapImagesDir, "map-images-dir", "map-images", "Directory with map images used for debug overlays")
	flag.StringVar(&outputDir, "output-dir", "output", "Output root directory (uses output/tagged-tilesets and writes output/replaymap-analyzer)")
	flag.StringVar(&onlyRunMap, "only-run-map", "", "Optional map name/filename stem to run a single map")
	flag.Parse()

	images, err := collectImageCandidates(mapImagesDir)
	if err != nil {
		fatalf("collect map images: %v", err)
	}
	if len(images) == 0 {
		fatalf("no map images found in %s", mapImagesDir)
	}
	replays, err := collectReplayCandidates(replaysDir)
	if err != nil {
		fatalf("collect replays: %v", err)
	}
	if len(replays) == 0 {
		fatalf("no replay files found in %s", replaysDir)
	}
	selector := normalizeKey(onlyRunMap)
	selected := selectBestRuns(images, replays, selector)
	if len(selected) == 0 {
		if selector != "" {
			fatalf("no replay/map-image match found for only-run-map=%q", onlyRunMap)
		}
		fatalf("no replay/map-image matches found")
	}
	keys := make([]string, 0, len(selected))
	for k := range selected {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	tagsRepoDir := filepath.Join(outputDir, "tagged-tilesets")
	mapOutDir := filepath.Join(outputDir, "replaymap-analyzer")
	if err := os.MkdirAll(mapOutDir, 0o755); err != nil {
		fatalf("mkdir output dir: %v", err)
	}

	ran := 0
	for _, key := range keys {
		run := selected[key]
		meta, parseErr := replay.ParseMapMetadata(run.ReplayPath)
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "skip %s: parse replay metadata: %v\n", run.ReplayPath, parseErr)
			continue
		}
		tags, loadErr := tiletags.LoadByTileSetKey(tagsRepoDir, meta.TilesetKey)
		if loadErr != nil {
			if errors.Is(loadErr, os.ErrNotExist) {
				fmt.Fprintf(os.Stderr, "skip %s: missing tile tags for %s under %s\n", key, meta.TilesetKey, tagsRepoDir)
				continue
			}
			fatalf("load tileset tags: %v", loadErr)
		}
		out, analyzeErr := replaymap.Analyze(meta, tags)
		if analyzeErr != nil {
			fmt.Fprintf(os.Stderr, "skip %s: analyze replay map: %v\n", key, analyzeErr)
			continue
		}
		outJSONPath := filepath.Join(mapOutDir, key+".json")
		outImagePath := filepath.Join(mapOutDir, key+"-overlay.png")
		if err := writeJSON(outJSONPath, out.Result); err != nil {
			fatalf("write output json: %v", err)
		}
		fmt.Printf("Wrote: %s\n", outJSONPath)

		base, decodeErr := decodeImage(run.ImagePath)
		if decodeErr != nil {
			fmt.Fprintf(os.Stderr, "skip %s overlay: decode map image: %v\n", key, decodeErr)
			continue
		}
		overlay := debugoverlay.ReplayMap(base, out.Debug)
		if err := writePNG(outImagePath, overlay); err != nil {
			fatalf("write overlay image: %v", err)
		}
		fmt.Printf("Wrote: %s\n", outImagePath)
		ran++
	}
	if ran == 0 {
		fatalf("replaymapanalyzer produced no successful runs")
	}
}

func decodeImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func collectReplayCandidates(replaysDir string) ([]replayCandidate, error) {
	files := []string{}
	err := filepath.WalkDir(replaysDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".rep") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	out := make([]replayCandidate, 0, len(files))
	for _, rp := range files {
		meta, parseErr := replay.ParseMapMetadata(rp)
		if parseErr != nil {
			continue
		}
		out = append(out, replayCandidate{Path: rp, Meta: meta})
	}
	return out, nil
}

func collectImageCandidates(mapImagesDir string) ([]imageCandidate, error) {
	entries, err := os.ReadDir(mapImagesDir)
	if err != nil {
		return nil, err
	}
	out := make([]imageCandidate, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
			continue
		}
		stemRaw := strings.TrimSuffix(name, filepath.Ext(name))
		stem := normalizeKey(stemRaw)
		if stem == "" {
			continue
		}
		out = append(out, imageCandidate{
			Path: filepath.Join(mapImagesDir, name),
			Stem: stem,
		})
	}
	sort.Slice(out, func(i int, j int) bool {
		return out[i].Stem < out[j].Stem
	})
	return out, nil
}

func selectBestRuns(images []imageCandidate, replays []replayCandidate, selector string) map[string]selectedRun {
	out := map[string]selectedRun{}
	for _, rc := range replays {
		if rc.Meta == nil {
			continue
		}
		bestStem := ""
		bestPath := ""
		bestScore := -math.MaxFloat64
		for _, img := range images {
			score := math.Max(
				matcher.ScoreName(img.Stem, rc.Meta.MapName),
				matcher.ScoreName(img.Stem, rc.Meta.MapDataName),
			)
			if score > bestScore {
				bestScore = score
				bestStem = img.Stem
				bestPath = img.Path
			}
		}
		if bestStem == "" {
			continue
		}
		if selector != "" && selector != bestStem &&
			selector != normalizeKey(rc.Meta.MapName) &&
			selector != normalizeKey(rc.Meta.MapDataName) {
			continue
		}
		prev, ok := out[bestStem]
		if ok && prev.Score >= bestScore {
			continue
		}
		out[bestStem] = selectedRun{
			ReplayPath: rc.Path,
			ImagePath:  bestPath,
			MapName:    rc.Meta.MapName,
			Score:      bestScore,
		}
	}
	return out
}

func normalizeKey(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	if v == "" {
		return ""
	}
	b := strings.Builder{}
	lastDash := false
	for _, r := range v {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
