package scmapanalyzer

import (
	"errors"

	"github.com/icza/screp/rep"
	"github.com/marianogappa/scmapanalyzer/internal/mapgfx"
	"github.com/marianogappa/scmapanalyzer/internal/replay"
)

// DefaultMapImageRenderOptions matches replay tooling: terrain plus mineral and geyser sprites.
var DefaultMapImageRenderOptions = mapgfx.RenderOptions{OverlayResources: true}

// MapImagePNGFromReplayFile parses the replay at path and returns a PNG of the map terrain
// (no analyzer debug overlays).
func MapImagePNGFromReplayFile(replayPath string) ([]byte, error) {
	if replayPath == "" {
		return nil, errors.New("replay path is required")
	}
	meta, err := replay.ParseMapMetadata(replayPath)
	if err != nil {
		return nil, err
	}
	return mapgfx.RenderMapPNGFromMetadata(meta, DefaultMapImageRenderOptions)
}

// MapImagePNGFromScrepReplay renders from a replay already parsed with github.com/icza/screp/repparser.
// Uses the replay header for map size and [rep.MapData] for tiles and resources.
func MapImagePNGFromScrepReplay(rep *rep.Replay) ([]byte, error) {
	if rep == nil {
		return nil, errors.New("replay is required")
	}
	meta, err := replay.MapMetadataFromReplay(rep, "")
	if err != nil {
		return nil, err
	}
	return mapgfx.RenderMapPNGFromMetadata(meta, DefaultMapImageRenderOptions)
}
