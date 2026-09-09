package scmapanalyzer

import (
	"errors"

	"github.com/icza/screp/rep"
	"github.com/marianogappa/scmapanalyzer/internal/mapgfx"
	"github.com/marianogappa/scmapanalyzer/internal/replay"
)

// DefaultMapImageRenderOptions matches replay tooling: terrain plus mineral and geyser sprites.
var DefaultMapImageRenderOptions = mapgfx.RenderOptions{OverlayResources: true}

// MapImageOptions controls the downscaled JPEG map render.
type MapImageOptions struct {
	MaxDim  int // pixel cap on the longest side; 0 means 2560
	Quality int // JPEG quality; 0 means 85
}

// TileRect is a crop region in map tiles.
type TileRect struct {
	X, Y, W, H int
}

// MapImageJPEGFromReplayFile parses the replay at path and returns a JPEG of the map terrain
// (no analyzer debug overlays), box-downsampled by the largest power of two that fits under
// opts.MaxDim on the longest side.
func MapImageJPEGFromReplayFile(replayPath string, opts MapImageOptions) ([]byte, error) {
	if replayPath == "" {
		return nil, errors.New("replay path is required")
	}
	meta, err := replay.ParseMapMetadata(replayPath)
	if err != nil {
		return nil, err
	}
	return mapgfx.RenderMapJPEGFromMetadata(meta, DefaultMapImageRenderOptions, jpegOptions(opts))
}

// MapImageJPEGFromScrepReplay renders from a replay already parsed with github.com/icza/screp/repparser.
// Uses the replay header for map size and [rep.MapData] for tiles and resources.
func MapImageJPEGFromScrepReplay(rep *rep.Replay, opts MapImageOptions) ([]byte, error) {
	if rep == nil {
		return nil, errors.New("replay is required")
	}
	meta, err := replay.MapMetadataFromReplay(rep, "")
	if err != nil {
		return nil, err
	}
	return mapgfx.RenderMapJPEGFromMetadata(meta, DefaultMapImageRenderOptions, jpegOptions(opts))
}

// MapImagePNGCropFromReplayFile parses the replay at path and returns a PNG of the given
// tile rect of the map terrain at native resolution (32 px per map tile), as a base for
// compositing sprites on top. Resource sprites straddling the rect edge are clipped to it.
func MapImagePNGCropFromReplayFile(replayPath string, r TileRect) ([]byte, error) {
	if replayPath == "" {
		return nil, errors.New("replay path is required")
	}
	meta, err := replay.ParseMapMetadata(replayPath)
	if err != nil {
		return nil, err
	}
	return mapgfx.RenderMapPNGCropFromMetadata(meta, DefaultMapImageRenderOptions, mapgfx.TileRect(r))
}

func jpegOptions(opts MapImageOptions) mapgfx.JPEGOptions {
	return mapgfx.JPEGOptions{MaxDim: opts.MaxDim, Quality: opts.Quality}
}
