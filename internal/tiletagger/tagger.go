package tiletagger

import (
	"errors"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/marianogappa/scmapanalyzer/internal/matcher"
	"github.com/marianogappa/scmapanalyzer/internal/model"
	"github.com/marianogappa/scmapanalyzer/internal/replay"
	"github.com/marianogappa/scmapanalyzer/internal/tiletags"
)

type Config struct {
	MapImagePath     string
	OverlayImagePath string
	ReplaysDir       string
	PerCellThreshold float64
	PerTileThreshold float64
	MinCellsPerTile  int
}

type Result struct {
	MatchedReplayPath string
	Tags              *tiletags.TileSetTags
}

type replayCandidate struct {
	Path     string
	Score    float64
	Metadata *model.MapMetadata
}

type accum struct {
	occ         int
	redCells    int
	purpleCells int
}

func Run(cfg Config) (*Result, error) {
	if cfg.MapImagePath == "" || cfg.OverlayImagePath == "" || cfg.ReplaysDir == "" {
		return nil, errors.New("map-image, overlay-image, and replays-dir are required")
	}
	if cfg.PerCellThreshold <= 0 || cfg.PerCellThreshold > 1 {
		return nil, errors.New("per-cell-threshold must be in (0,1]")
	}
	if cfg.PerTileThreshold <= 0 || cfg.PerTileThreshold > 1 {
		return nil, errors.New("per-tile-threshold must be in (0,1]")
	}
	if cfg.MinCellsPerTile <= 0 {
		return nil, errors.New("min-cells-per-tile must be > 0")
	}

	replayPath, meta, err := matchReplay(cfg.ReplaysDir, cfg.MapImagePath)
	if err != nil {
		return nil, err
	}
	baseImage, err := decodeImage(cfg.MapImagePath)
	if err != nil {
		return nil, err
	}
	overlayImage, err := decodeImage(cfg.OverlayImagePath)
	if err != nil {
		return nil, err
	}
	if baseImage.Bounds() != overlayImage.Bounds() {
		return nil, errors.New("map image and overlay image dimensions differ")
	}
	if len(meta.Tiles) != meta.WidthTiles*meta.HeightTiles {
		return nil, errors.New("replay tile grid mismatch")
	}

	wallIDs, rampIDs := classifyTiles(
		meta.Tiles,
		meta.WidthTiles,
		meta.HeightTiles,
		overlayImage,
		cfg.PerCellThreshold,
		cfg.PerTileThreshold,
		cfg.MinCellsPerTile,
	)
	tags := &tiletags.TileSetTags{
		TileSetID:   meta.Tileset,
		TileSetName: meta.TilesetName,
		TileSetKey:  meta.TilesetKey,
		SourceMap:   filepath.Base(cfg.MapImagePath),
		WallTileIDs: wallIDs,
		RampTileIDs: rampIDs,
	}
	return &Result{
		MatchedReplayPath: replayPath,
		Tags:              tags,
	}, nil
}

func matchReplay(replaysDir string, mapImagePath string) (string, *model.MapMetadata, error) {
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
		return "", nil, err
	}
	if len(files) == 0 {
		return "", nil, errors.New("no replay files found")
	}
	imageName := strings.TrimSuffix(filepath.Base(mapImagePath), filepath.Ext(mapImagePath))
	best := replayCandidate{Score: -1}
	for _, rp := range files {
		meta, parseErr := replay.ParseMapMetadata(rp)
		if parseErr != nil {
			continue
		}
		score := math.Max(
			matcher.ScoreName(imageName, meta.MapName),
			matcher.ScoreName(imageName, meta.MapDataName),
		)
		if score > best.Score {
			best = replayCandidate{
				Path:     rp,
				Score:    score,
				Metadata: meta,
			}
		}
	}
	if best.Metadata == nil {
		return "", nil, errors.New("could not parse any replay metadata")
	}
	return best.Path, best.Metadata, nil
}

func classifyTiles(
	tiles []uint16,
	widthTiles int,
	heightTiles int,
	overlay image.Image,
	perCellThreshold float64,
	perTileThreshold float64,
	minCellsPerTile int,
) ([]uint16, []uint16) {
	stats := map[uint16]*accum{}
	stepX := float64(overlay.Bounds().Dx()) / float64(widthTiles)
	stepY := float64(overlay.Bounds().Dy()) / float64(heightTiles)

	for y := 0; y < heightTiles; y++ {
		for x := 0; x < widthTiles; x++ {
			idx := y*widthTiles + x
			tid := tiles[idx]
			a := stats[tid]
			if a == nil {
				a = &accum{}
				stats[tid] = a
			}
			a.occ++
			rect := tileRect(x, y, stepX, stepY, overlay.Bounds())
			redRatio, purpleRatio := cellColorCoverage(overlay, rect)
			if redRatio >= perCellThreshold {
				a.redCells++
			}
			if purpleRatio >= perCellThreshold {
				a.purpleCells++
			}
		}
	}

	wallIDs := make([]uint16, 0)
	rampIDs := make([]uint16, 0)
	for tileID, a := range stats {
		if a.occ < minCellsPerTile {
			continue
		}
		redShare := float64(a.redCells) / float64(a.occ)
		purpleShare := float64(a.purpleCells) / float64(a.occ)
		if redShare >= perTileThreshold {
			wallIDs = append(wallIDs, tileID)
		}
		if purpleShare >= perTileThreshold {
			rampIDs = append(rampIDs, tileID)
		}
	}
	sort.Slice(wallIDs, func(i int, j int) bool { return wallIDs[i] < wallIDs[j] })
	sort.Slice(rampIDs, func(i int, j int) bool { return rampIDs[i] < rampIDs[j] })
	return wallIDs, rampIDs
}

func cellColorCoverage(img image.Image, rect image.Rectangle) (float64, float64) {
	total := 0
	red := 0
	purple := 0
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			total++
			r, g, b, _ := img.At(x, y).RGBA()
			c := color.RGBA{
				R: uint8(r >> 8),
				G: uint8(g >> 8),
				B: uint8(b >> 8),
				A: 255,
			}
			if isBrightRed(c) {
				red++
			}
			if isBrightPurple(c) {
				purple++
			}
		}
	}
	if total == 0 {
		return 0, 0
	}
	return float64(red) / float64(total), float64(purple) / float64(total)
}

func isBrightRed(c color.RGBA) bool {
	return c.R >= 200 && c.G <= 100 && c.B <= 100
}

func isBrightPurple(c color.RGBA) bool {
	return c.R >= 140 && c.B >= 140 && c.G <= 120
}

func tileRect(x int, y int, stepX float64, stepY float64, bounds image.Rectangle) image.Rectangle {
	x0 := bounds.Min.X + int(stepX*float64(x)+0.5)
	x1 := bounds.Min.X + int(stepX*float64(x+1)+0.5)
	y0 := bounds.Min.Y + int(stepY*float64(y)+0.5)
	y1 := bounds.Min.Y + int(stepY*float64(y+1)+0.5)
	if x1 <= x0 {
		x1 = x0 + 1
	}
	if y1 <= y0 {
		y1 = y0 + 1
	}
	return image.Rect(x0, y0, x1, y1).Intersect(bounds)
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
