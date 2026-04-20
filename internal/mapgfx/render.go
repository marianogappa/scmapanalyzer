package mapgfx

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io/fs"
	"path"
	"strings"

	"github.com/marianogappa/scmapanalyzer/internal/mapgfx/grp"
	"github.com/marianogappa/scmapanalyzer/internal/mapgfx/tileset"
)

// MapData is the minimal replay-side map description for rendering.
type MapData struct {
	TileSet  string
	Width    int
	Height   int
	Tiles    []uint16
	Minerals []Point
	Geysers  []Point
}

type Point struct {
	X int
	Y int
}

type RenderOptions struct {
	OverlayResources bool
}

type spriteManifest struct {
	Sprites map[string]string `json:"sprites"`
}

func loadSpriteManifest() (*spriteManifest, error) {
	b, err := fs.ReadFile(assets, path.Join("data", "sprites", "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("read sprites manifest: %w", err)
	}
	var m spriteManifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("parse sprites manifest: %w", err)
	}
	if len(m.Sprites) == 0 {
		return nil, errors.New("sprites manifest empty")
	}
	for k, v := range m.Sprites {
		m.Sprites[k] = path.Clean(v)
	}
	return &m, nil
}

func spriteImageRGBA(spriteName string, palette color.Palette) (*image.RGBA, error) {
	m, err := loadSpriteManifest()
	if err != nil {
		return nil, err
	}
	rel, ok := m.Sprites[strings.ToLower(spriteName)]
	if !ok {
		return nil, fmt.Errorf("sprite not found: %s", spriteName)
	}
	b, err := fs.ReadFile(assets, path.Join("data", "sprites", rel))
	if err != nil {
		return nil, fmt.Errorf("read sprite grp %s: %w", rel, err)
	}
	return grp.DecodeFrameRGBA(b, 0, palette)
}

// RenderMapPNG renders the map to PNG bytes (32 px per map tile). Resource coordinates are pixels.
func RenderMapPNG(md MapData, opts RenderOptions) ([]byte, error) {
	if md.Width <= 0 || md.Height <= 0 || len(md.Tiles) == 0 {
		return nil, errors.New("invalid map metadata")
	}
	folder, err := TilesetAssetFolderFromReplay(md.TileSet)
	if err != nil {
		return nil, err
	}
	raw, err := tileset.LoadFromFS(assets, folder)
	if err != nil {
		return nil, err
	}
	base, err := tileset.RenderMapToPaletted(raw, md.Width, md.Height, md.Tiles)
	if err != nil {
		return nil, err
	}
	if !opts.OverlayResources {
		return encodePNG(base)
	}

	rgba := image.NewRGBA(base.Bounds())
	draw.Draw(rgba, rgba.Bounds(), base, image.Point{}, draw.Src)
	pal, err := tileset.PaletteFromWPE(raw.WPE)
	if err != nil {
		return nil, err
	}

	mineralNames := []string{"neutral/min01", "neutral/min02", "neutral/min03"}
	for i, p := range md.Minerals {
		spr, err := spriteImageRGBA(mineralNames[i%len(mineralNames)], pal)
		if err != nil {
			return nil, err
		}
		dx, dy := spr.Bounds().Dx(), spr.Bounds().Dy()
		min := image.Pt(p.X-dx/2, p.Y-dy/2)
		dst := image.Rectangle{Min: min, Max: min.Add(image.Pt(dx, dy))}
		draw.Draw(rgba, dst, spr, image.Point{}, draw.Over)
	}
	for _, p := range md.Geysers {
		spr, err := spriteImageRGBA("neutral/geyser", pal)
		if err != nil {
			return nil, err
		}
		dx, dy := spr.Bounds().Dx(), spr.Bounds().Dy()
		min := image.Pt(p.X-dx/2, p.Y-dy/2)
		dst := image.Rectangle{Min: min, Max: min.Add(image.Pt(dx, dy))}
		draw.Draw(rgba, dst, spr, image.Point{}, draw.Over)
	}
	return encodePNG(rgba)
}

func encodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
