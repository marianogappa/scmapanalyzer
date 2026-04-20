package replay

import (
	"errors"
	"fmt"

	"github.com/icza/screp/rep"
	"github.com/icza/screp/repparser"
	"github.com/marianogappa/scmapanalyzer/internal/model"
)

// MapMetadataFromReplay builds [model.MapMetadata] from an already-parsed replay.
// replayPath is used only in error messages; pass empty when not applicable.
func MapMetadataFromReplay(rep *rep.Replay, replayPath string) (*model.MapMetadata, error) {
	if rep == nil || rep.Header == nil {
		return nil, errors.New("missing replay header")
	}
	if rep.MapData == nil {
		if replayPath != "" {
			return nil, fmt.Errorf("replay has no map data for %q", replayPath)
		}
		return nil, errors.New("replay has no map data")
	}

	tileset := 0
	tileSetMissing := rep.MapData.TileSetMissing
	if rep.MapData.TileSet != nil {
		tileset = int(rep.MapData.TileSet.ID)
	}
	tileSetMeta := TileSetMetadata(tileset, tileSetMissing)

	meta := &model.MapMetadata{
		ReplayPath:     replayPath,
		MapName:        rep.Header.Map,
		MapDataName:    rep.MapData.Name,
		WidthTiles:     int(rep.Header.MapWidth),
		HeightTiles:    int(rep.Header.MapHeight),
		Tileset:        tileset,
		TilesetName:    tileSetMeta.Name,
		TilesetKey:     tileSetMeta.Key,
		TileSetMissing: tileSetMissing,
		Tiles:          rep.MapData.Tiles,
	}
	expected := meta.WidthTiles * meta.HeightTiles
	if got := len(meta.Tiles); got > expected {
		meta.Tiles = meta.Tiles[:expected]
	} else if got < expected {
		if replayPath != "" {
			return nil, fmt.Errorf("replay tile grid mismatch for %q: got %d tiles, want %d (%dx%d)", replayPath, got, expected, meta.WidthTiles, meta.HeightTiles)
		}
		return nil, fmt.Errorf("replay tile grid mismatch: got %d tiles, want %d (%dx%d)", got, expected, meta.WidthTiles, meta.HeightTiles)
	}
	for _, m := range rep.MapData.MineralFields {
		meta.MineralFields = append(meta.MineralFields, model.MapResource{
			X: int(m.X),
			Y: int(m.Y),
		})
	}
	for _, g := range rep.MapData.Geysers {
		meta.Geysers = append(meta.Geysers, model.MapResource{
			X: int(g.X),
			Y: int(g.Y),
		})
	}
	for _, s := range rep.MapData.StartLocations {
		meta.StartLocations = append(meta.StartLocations, model.StartLocation{
			X:      int(s.X),
			Y:      int(s.Y),
			SlotID: s.SlotID,
		})
	}
	return meta, nil
}

func ParseMapMetadata(path string) (*model.MapMetadata, error) {
	rep, err := repparser.ParseFile(path)
	if err != nil {
		return nil, err
	}
	meta, err := MapMetadataFromReplay(rep, path)
	if err != nil {
		return nil, err
	}
	meta.ReplayPath = path
	return meta, nil
}
