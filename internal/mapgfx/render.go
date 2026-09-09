package mapgfx

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io/fs"
	"path"
	"strings"

	"github.com/marianogappa/scmapanalyzer/internal/mapgfx/sprite"
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
		return nil, fmt.Errorf("read sprite %s: %w", rel, err)
	}
	return sprite.DecodeRGBA(b, palette)
}

// TileRect is a crop region in map tiles.
type TileRect struct {
	X, Y, W, H int
}

// JPEGOptions controls the downscaled JPEG render.
type JPEGOptions struct {
	MaxDim  int // pixel cap on the longest side; 0 means 2560
	Quality int // JPEG quality; 0 means 85
}

const (
	defaultJPEGMaxDim  = 2560
	defaultJPEGQuality = 85
)

// RenderMapPNG renders the map to PNG bytes (32 px per map tile). Resource coordinates are pixels.
func RenderMapPNG(md MapData, opts RenderOptions) ([]byte, error) {
	img, err := renderMapRegion(md, opts, TileRect{X: 0, Y: 0, W: md.Width, H: md.Height})
	if err != nil {
		return nil, err
	}
	return encodePNG(img)
}

// RenderMapPNGCrop renders only the given tile rect to PNG bytes at native 32 px per tile.
// Resource sprites straddling the rect edge are clipped to it.
func RenderMapPNGCrop(md MapData, opts RenderOptions, r TileRect) ([]byte, error) {
	img, err := renderMapRegion(md, opts, r)
	if err != nil {
		return nil, err
	}
	return encodePNG(img)
}

// RenderMapJPEG renders the map, box-downsamples it by the largest power of two
// that fits under jopts.MaxDim, and encodes JPEG. The 4096x4096 native image is
// never PNG-encoded.
func RenderMapJPEG(md MapData, opts RenderOptions, jopts JPEGOptions) ([]byte, error) {
	img, err := renderMapRegion(md, opts, TileRect{X: 0, Y: 0, W: md.Width, H: md.Height})
	if err != nil {
		return nil, err
	}
	maxDim := jopts.MaxDim
	if maxDim == 0 {
		maxDim = defaultJPEGMaxDim
	}
	if maxDim < 0 {
		return nil, fmt.Errorf("invalid MaxDim: %d", maxDim)
	}
	quality := jopts.Quality
	if quality == 0 {
		quality = defaultJPEGQuality
	}
	if quality < 1 || quality > 100 {
		return nil, fmt.Errorf("invalid Quality: %d", quality)
	}
	if factor := downsampleFactor(img.Bounds().Dx(), img.Bounds().Dy(), maxDim); factor > 1 {
		img = boxDownsample(img, factor)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func renderMapRegion(md MapData, opts RenderOptions, r TileRect) (image.Image, error) {
	if md.Width <= 0 || md.Height <= 0 || len(md.Tiles) == 0 {
		return nil, errors.New("invalid map metadata")
	}
	folder, err := TilesetAssetFolderFromReplay(md.TileSet)
	if err != nil {
		return nil, err
	}
	pack, err := tileset.LoadPackFromFS(assets, folder)
	if err != nil {
		return nil, err
	}
	base, err := tileset.RenderPackRegionToPaletted(pack, md.Width, md.Height, md.Tiles, r.X, r.Y, r.W, r.H)
	if err != nil {
		return nil, err
	}
	if !opts.OverlayResources {
		return base, nil
	}

	// Keep map-pixel coordinates so sprite draws land at absolute resource
	// positions and draw.Draw clips them to the crop for free.
	rgba := image.NewRGBA(base.Bounds())
	draw.Draw(rgba, rgba.Bounds(), base, base.Bounds().Min, draw.Src)
	pal := pack.Palette

	mineralNames := []string{"neutral/min01", "neutral/min02", "neutral/min03"}
	for i, p := range md.Minerals {
		spr, err := spriteImageRGBA(mineralNames[i%len(mineralNames)], pal)
		if err != nil {
			return nil, err
		}
		drawSpriteCentered(rgba, spr, p)
	}
	for _, p := range md.Geysers {
		spr, err := spriteImageRGBA("neutral/geyser", pal)
		if err != nil {
			return nil, err
		}
		drawSpriteCentered(rgba, spr, p)
	}
	return rgba, nil
}

func drawSpriteCentered(dst *image.RGBA, spr *image.RGBA, p Point) {
	dx, dy := spr.Bounds().Dx(), spr.Bounds().Dy()
	min := image.Pt(p.X-dx/2, p.Y-dy/2)
	rect := image.Rectangle{Min: min, Max: min.Add(image.Pt(dx, dy))}
	draw.Draw(dst, rect, spr, image.Point{}, draw.Over)
}

func encodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
