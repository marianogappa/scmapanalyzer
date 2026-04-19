package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/marianogappa/scmapanalyzer/internal/tiletagger"
	"github.com/marianogappa/scmapanalyzer/internal/tiletags"
)

type runOutput struct {
	MapImagePath      string                `json:"map_image_path"`
	OverlayImagePath  string                `json:"overlay_image_path"`
	MatchedReplayPath string                `json:"matched_replay_path"`
	Tags              *tiletags.TileSetTags `json:"tags"`
	RepositoryPath    string                `json:"repository_path"`
}

type output struct {
	Count int         `json:"count"`
	Runs  []runOutput `json:"runs"`
}

func main() {
	var (
		mapImagesDir string
		overlaysDir  string
		replaysDir   string
		outputDir    string
		onlyRunMap   string
	)
	flag.StringVar(&mapImagesDir, "map-images-dir", "map-images", "Directory with base map images")
	flag.StringVar(&overlaysDir, "overlays-dir", "sample-map-masks", "Directory with red/purple overlay images")
	flag.StringVar(&replaysDir, "replays-dir", "replays", "Directory to search for matching replay")
	flag.StringVar(&outputDir, "output-dir", "output", "Output root directory (writes tagged-tilesets and tiletagger-result.json)")
	flag.StringVar(&onlyRunMap, "only-run-map", "", "Optional map name/filename stem to run a single map")
	flag.Parse()

	tagsRepoDir := filepath.Join(outputDir, "tagged-tilesets")
	outJSONPath := filepath.Join(outputDir, "tiletagger-result.json")

	mapImages, err := collectImageIndex(mapImagesDir)
	if err != nil {
		fatalf("collect map images: %v", err)
	}
	overlays, err := collectImageIndex(overlaysDir)
	if err != nil {
		fatalf("collect overlays: %v", err)
	}
	if len(mapImages) == 0 {
		fatalf("no map images found in %s", mapImagesDir)
	}
	if len(overlays) == 0 {
		fatalf("no overlay images found in %s", overlaysDir)
	}

	keys := make([]string, 0)
	selector := normalizeKey(onlyRunMap)
	for k := range mapImages {
		if _, ok := overlays[k]; !ok {
			continue
		}
		if selector != "" && k != selector {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		if selector != "" {
			fatalf("no map/overlay pair found for only-run-map=%q", onlyRunMap)
		}
		fatalf("no map/overlay pairs found between %s and %s", mapImagesDir, overlaysDir)
	}

	runs := make([]runOutput, 0, len(keys))
	for _, key := range keys {
		mapImagePath := mapImages[key]
		overlayImagePath := overlays[key]
		res, runErr := tiletagger.Run(tiletagger.Config{
			MapImagePath:     mapImagePath,
			OverlayImagePath: overlayImagePath,
			ReplaysDir:       replaysDir,
			PerCellThreshold: 0.45,
			PerTileThreshold: 0.45,
			MinCellsPerTile:  2,
		})
		if runErr != nil {
			fmt.Fprintf(os.Stderr, "skip %s: %v\n", key, runErr)
			continue
		}

		// Incremental tags in output/tagged-tilesets:
		// merge previous IDs with newly detected IDs for this tileset key.
		existing, loadErr := tiletags.LoadByTileSetKey(tagsRepoDir, res.Tags.TileSetKey)
		if loadErr == nil && existing != nil {
			res.Tags.WallTileIDs = mergeIDs(existing.WallTileIDs, res.Tags.WallTileIDs, res.Tags.WalkableIDs)
			res.Tags.RampTileIDs = mergeIDs(existing.RampTileIDs, res.Tags.RampTileIDs, res.Tags.WalkableIDs)
		}
		if loadErr != nil && !errors.Is(loadErr, os.ErrNotExist) {
			fatalf("load existing tags for merge (%s): %v", res.Tags.TileSetKey, loadErr)
		}

		repoPath, saveErr := tiletags.Save(tagsRepoDir, res.Tags)
		if saveErr != nil {
			fatalf("save tags repo entry (%s): %v", res.Tags.TileSetKey, saveErr)
		}
		fmt.Printf("Wrote: %s\n", repoPath)
		runs = append(runs, runOutput{
			MapImagePath:      mapImagePath,
			OverlayImagePath:  overlayImagePath,
			MatchedReplayPath: res.MatchedReplayPath,
			Tags:              res.Tags,
			RepositoryPath:    repoPath,
		})
	}
	if len(runs) == 0 {
		fatalf("tiletagger produced no successful runs")
	}

	out := output{
		Count: len(runs),
		Runs:  runs,
	}
	if err := os.MkdirAll(filepath.Dir(outJSONPath), 0o755); err != nil {
		fatalf("mkdir output dir: %v", err)
	}
	if err := writeJSON(outJSONPath, out); err != nil {
		fatalf("write output json: %v", err)
	}
	fmt.Printf("Wrote: %s\n", outJSONPath)
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}

func mergeIDs(a []uint16, b []uint16, walkable []uint16) []uint16 {
	seen := map[uint16]struct{}{}
	for _, id := range a {
		seen[id] = struct{}{}
	}
	for _, id := range b {
		seen[id] = struct{}{}
	}
	for _, id := range walkable {
		delete(seen, id)
	}
	merged := make([]uint16, 0, len(seen))
	for id := range seen {
		merged = append(merged, id)
	}
	sort.Slice(merged, func(i int, j int) bool { return merged[i] < merged[j] })
	return merged
}

func collectImageIndex(dir string) (map[string]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
			continue
		}
		stem := normalizeKey(strings.TrimSuffix(name, filepath.Ext(name)))
		if stem == "" {
			continue
		}
		if _, exists := out[stem]; exists {
			continue
		}
		out[stem] = filepath.Join(dir, name)
	}
	return out, nil
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
	out := strings.Trim(b.String(), "-")
	return out
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
