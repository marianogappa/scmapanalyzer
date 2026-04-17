package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/marianogappa/scmapanalyzer/internal/tiletagger"
	"github.com/marianogappa/scmapanalyzer/internal/tiletags"
)

type output struct {
	MatchedReplayPath string                `json:"matched_replay_path"`
	Tags              *tiletags.TileSetTags `json:"tags"`
	RepositoryPath    string                `json:"repository_path"`
}

func main() {
	var (
		mapImagePath     string
		overlayImagePath string
		replaysDir       string
		tagsRepoDir      string
		outJSONPath      string
	)
	flag.StringVar(&mapImagePath, "map-image", "", "Path to the base map image")
	flag.StringVar(&overlayImagePath, "overlay-image", "", "Path to map image with red/purple wall/ramp overlays")
	flag.StringVar(&replaysDir, "replays-dir", "replays", "Directory to search for matching replay")
	flag.StringVar(&tagsRepoDir, "tags-repo", filepath.Join("output", "tagged-tilesets"), "Directory where per-tileset tagging JSON files are stored")
	flag.StringVar(&outJSONPath, "out-json", filepath.Join("output", "tiletagger-result.json"), "Output JSON path for this run")
	flag.Parse()

	res, err := tiletagger.Run(tiletagger.Config{
		MapImagePath:     mapImagePath,
		OverlayImagePath: overlayImagePath,
		ReplaysDir:       replaysDir,
		PerCellThreshold: 0.45,
		PerTileThreshold: 0.55,
		MinCellsPerTile:  2,
	})
	if err != nil {
		fatalf("run tiletagger: %v", err)
	}

	// If tags for this tileset already exist, augment instead of overwrite:
	// keep all previously tagged IDs and add any newly detected IDs.
	existing, err := tiletags.LoadByTileSetKey(tagsRepoDir, res.Tags.TileSetKey)
	if err == nil && existing != nil {
		res.Tags.WallTileIDs = mergeIDs(existing.WallTileIDs, res.Tags.WallTileIDs)
		res.Tags.RampTileIDs = mergeIDs(existing.RampTileIDs, res.Tags.RampTileIDs)
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		fatalf("load existing tags for merge: %v", err)
	}

	repoPath, err := tiletags.Save(tagsRepoDir, res.Tags)
	if err != nil {
		fatalf("save tags repo entry: %v", err)
	}
	out := output{
		MatchedReplayPath: res.MatchedReplayPath,
		Tags:              res.Tags,
		RepositoryPath:    repoPath,
	}
	if err := os.MkdirAll(filepath.Dir(outJSONPath), 0o755); err != nil {
		fatalf("mkdir output dir: %v", err)
	}
	if err := writeJSON(outJSONPath, out); err != nil {
		fatalf("write output json: %v", err)
	}
	fmt.Printf("Wrote: %s\n", repoPath)
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

func mergeIDs(a []uint16, b []uint16) []uint16 {
	seen := map[uint16]bool{}
	for _, id := range a {
		seen[id] = true
	}
	for _, id := range b {
		seen[id] = true
	}
	merged := make([]uint16, 0, len(seen))
	for id := range seen {
		merged = append(merged, id)
	}
	sort.Slice(merged, func(i int, j int) bool { return merged[i] < merged[j] })
	return merged
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
