package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"os"
	"path/filepath"

	debugoverlay "github.com/marianogappa/scmapanalyzer/internal/debug/overlay"
	"github.com/marianogappa/scmapanalyzer/internal/replay"
	"github.com/marianogappa/scmapanalyzer/internal/tiletags"
	"github.com/marianogappa/scmapanalyzer/replaymap"
)

func main() {
	var (
		replayPath  string
		tagsRepoDir string
		outJSONPath string
		mapImage    string
		outImage    string
	)
	flag.StringVar(&replayPath, "replay", "", "Replay file to analyze")
	flag.StringVar(&tagsRepoDir, "tags-repo", filepath.Join("output", "tagged-tilesets"), "Directory of per-tileset tag files")
	flag.StringVar(&outJSONPath, "out-json", filepath.Join("output", "replaymap-analyzer.json"), "Output JSON path")
	flag.StringVar(&mapImage, "map-image", "", "Optional map image path for debug overlay")
	flag.StringVar(&outImage, "out-image", filepath.Join("output", "replaymap-analyzer-overlay.png"), "Optional debug overlay output path (requires -map-image)")
	flag.Parse()

	if replayPath == "" {
		fatalf("replay is required")
	}

	meta, err := replay.ParseMapMetadata(replayPath)
	if err != nil {
		fatalf("parse replay metadata: %v", err)
	}
	tags, err := tiletags.LoadByTileSetKey(tagsRepoDir, meta.TilesetKey)
	if err != nil {
		fatalf("load tileset tags: %v", err)
	}
	out, err := replaymap.Analyze(meta, tags)
	if err != nil {
		fatalf("analyze replay map: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(outJSONPath), 0o755); err != nil {
		fatalf("mkdir output dir: %v", err)
	}
	if err := writeJSON(outJSONPath, out.Result); err != nil {
		fatalf("write output json: %v", err)
	}
	fmt.Printf("Wrote: %s\n", outJSONPath)

	if mapImage != "" {
		base, err := decodeImage(mapImage)
		if err != nil {
			fatalf("decode map image: %v", err)
		}
		overlay := debugoverlay.ReplayMap(base, out.Debug)
		if err := os.MkdirAll(filepath.Dir(outImage), 0o755); err != nil {
			fatalf("mkdir image dir: %v", err)
		}
		if err := writePNG(outImage, overlay); err != nil {
			fatalf("write overlay image: %v", err)
		}
		fmt.Printf("Wrote: %s\n", outImage)
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
