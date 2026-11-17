// basechokes renders, per map, every detected base area and its chokes over
// the terrain, for reviewing shape and choke detection.
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

func main() {
	replaysDir := flag.String("replays-dir", "replays", "directory scanned recursively for .rep files")
	outDir := flag.String("out", "output/basechokes", "output directory")
	maps := flag.String("maps", "", "comma-separated substrings of map names to render (empty = all)")
	px := flag.Int("px", 2, "output pixels per minitile")
	flag.Parse()

	metas, err := uniqueMaps(*replaysDir, *maps)
	if err != nil {
		fatalf("scan replays: %v", err)
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fatalf("mkdir: %v", err)
	}
	keys := make([]string, 0, len(metas))
	for k := range metas {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		meta := metas[key]
		out, err := replaymap.Analyze(meta)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skip %s: %v\n", key, err)
			continue
		}
		pngBytes, err := mapgfx.RenderMapPNGFromMetadata(meta, mapgfx.RenderOptions{OverlayResources: true})
		if err != nil {
			fatalf("%s: render: %v", key, err)
		}
		terrain, err := png.Decode(bytes.NewReader(pngBytes))
		if err != nil {
			fatalf("%s: decode: %v", key, err)
		}
		d := out.Debug
		masks := append(append([][]bool{}, d.StartMasks...), d.ExpaMasks...)
		img := debugoverlay.BaseShapes(terrain, out.Result, masks, d.WidthMinitiles, d.HeightMinitiles, *px, true)
		writePNG(filepath.Join(*outDir, key+".png"), img)
		b, _ := json.MarshalIndent(out.Result, "", "  ")
		_ = os.WriteFile(filepath.Join(*outDir, key+".json"), b, 0o644)
		fmt.Println("wrote", key)
	}
}

func uniqueMaps(dir, filter string) (map[string]*model.MapMetadata, error) {
	var wants []string
	for _, f := range strings.Split(filter, ",") {
		if f = strings.TrimSpace(f); f != "" {
			wants = append(wants, key(f))
		}
	}
	var files []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.EqualFold(filepath.Ext(p), ".rep") {
			files = append(files, p)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	out := map[string]*model.MapMetadata{}
	for _, f := range files {
		meta, err := replay.ParseMapMetadata(f)
		if err != nil {
			continue
		}
		k := key(meta.MapName)
		if _, ok := out[k]; ok || k == "" {
			continue
		}
		match := len(wants) == 0
		for _, w := range wants {
			if strings.Contains(strings.ReplaceAll(k, "-", ""), strings.ReplaceAll(w, "-", "")) {
				match = true
			}
		}
		if match {
			out[k] = meta
		}
	}
	return out, nil
}

func key(v string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(v)) {
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

func writePNG(path string, img image.Image) {
	f, err := os.Create(path)
	if err != nil {
		fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		fatalf("encode %s: %v", path, err)
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
