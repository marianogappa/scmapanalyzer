package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"

	debugoverlay "github.com/marianogappa/scmapanalyzer/internal/debug/overlay"
	"github.com/marianogappa/scmapanalyzer/internal/mapgfx"
	"github.com/marianogappa/scmapanalyzer/internal/model"
	"github.com/marianogappa/scmapanalyzer/internal/replay"
	"github.com/marianogappa/scmapanalyzer/replaymap"
)

type replayCandidate struct {
	Path string
	Meta *model.MapMetadata
}

func main() {
	var (
		replaysDir string
		outputDir  string
		onlyRunMap string
	)
	flag.StringVar(&replaysDir, "replays-dir", "replays", "Directory to scan recursively for .rep files")
	flag.StringVar(&outputDir, "output-dir", "output", "Output root directory (writes output/replaymap-analyzer)")
	flag.StringVar(&onlyRunMap, "only-run-map", "", "Optional map name/filename stem to run a single map")
	flag.Parse()

	replays, err := collectReplayCandidates(replaysDir)
	if err != nil {
		fatalf("collect replays: %v", err)
	}
	if len(replays) == 0 {
		fatalf("no replay files found in %s", replaysDir)
	}
	selector := normalizeKey(onlyRunMap)
	selected := selectUniqueMapReplays(replays, selector)
	if len(selected) == 0 {
		if selector != "" {
			fatalf("no replay found for only-run-map=%q", onlyRunMap)
		}
		fatalf("no replay candidates after dedupe")
	}
	keys := make([]string, 0, len(selected))
	for k := range selected {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	mapOutDir := filepath.Join(outputDir, "replaymap-analyzer")
	if err := os.MkdirAll(mapOutDir, 0o755); err != nil {
		fatalf("mkdir output dir: %v", err)
	}

	var digest []digestEntry
	for _, key := range keys {
		rc := selected[key]
		meta, parseErr := replay.ParseMapMetadata(rc.Path)
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "skip %s: parse replay metadata: %v\n", rc.Path, parseErr)
			continue
		}
		out, analyzeErr := replaymap.Analyze(meta)
		if analyzeErr != nil {
			fmt.Fprintf(os.Stderr, "skip %s: analyze replay map: %v\n", key, analyzeErr)
			continue
		}
		outJSONPath := filepath.Join(mapOutDir, key+".json")
		outImagePath := filepath.Join(mapOutDir, key+"-overlay.png")
		if err := writeJSON(outJSONPath, out.Result); err != nil {
			fatalf("write output json: %v", err)
		}
		dupes := replaymap.DuplicateNamesInResult(out.Result)
		if len(dupes) > 0 {
			fmt.Fprintf(os.Stderr, "WARNING: duplicate base names in map %s: %v\n", key, dupes)
		}
		digest = append(digest, digestEntry{Key: key, Result: out.Result, Dupes: dupes})
		fmt.Printf("Wrote: %s\n", outJSONPath)

		pngBytes, rendErr := mapgfx.RenderMapPNGFromMetadata(meta, mapgfx.RenderOptions{OverlayResources: true})
		if rendErr != nil {
			fmt.Fprintf(os.Stderr, "skip %s overlay: render map: %v\n", key, rendErr)
			continue
		}
		base, decodeErr := png.Decode(bytes.NewReader(pngBytes))
		if decodeErr != nil {
			fmt.Fprintf(os.Stderr, "skip %s overlay: decode rendered png: %v\n", key, decodeErr)
			continue
		}
		overlay := debugoverlay.ReplayMap(base, out.Result, out.Debug)
		if err := writePNG(outImagePath, overlay); err != nil {
			fatalf("write overlay image: %v", err)
		}
		fmt.Printf("Wrote: %s\n", outImagePath)
	}
	if len(digest) == 0 {
		fatalf("replaymapanalyzer produced no JSON output")
	}
	digestPath := filepath.Join(mapOutDir, "bases-digest.md")
	digestToWrite := digest
	if selector != "" {
		var derr error
		digestToWrite, derr = digestFromOutputJSON(mapOutDir)
		if derr != nil {
			fatalf("rebuild digest from output json: %v", derr)
		}
	}
	if err := writeBasesDigest(digestPath, digestToWrite); err != nil {
		fatalf("write bases digest: %v", err)
	}
	fmt.Printf("Wrote: %s\n", digestPath)
}

func selectUniqueMapReplays(replays []replayCandidate, selector string) map[string]replayCandidate {
	out := make(map[string]replayCandidate)
	for _, rc := range replays {
		if rc.Meta == nil {
			continue
		}
		key := normalizeKey(rc.Meta.MapName)
		if key == "" {
			key = normalizeKey(rc.Meta.MapDataName)
		}
		if key == "" {
			continue
		}
		if selector != "" && selector != key &&
			selector != normalizeKey(rc.Meta.MapDataName) {
			continue
		}
		if _, ok := out[key]; ok {
			continue
		}
		out[key] = rc
	}
	return out
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
