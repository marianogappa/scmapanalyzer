package replay

import (
	"fmt"

	"github.com/icza/screp/repparser"
	"github.com/marianogappa/scmapanalyzer/internal/model"
)

func ParseMapMetadata(path string) (*model.MapMetadata, error) {
	rep, err := repparser.ParseFile(path)
	if err != nil {
		return nil, err
	}
	if rep == nil || rep.Header == nil {
		return nil, fmt.Errorf("missing replay header for %q", path)
	}
	if rep.MapData == nil {
		return nil, fmt.Errorf("replay has no map data for %q", path)
	}

	tileset := 0
	tileSetMissing := rep.MapData.TileSetMissing
	if rep.MapData.TileSet != nil {
		tileset = int(rep.MapData.TileSet.ID)
	}
	tileSetMeta := TileSetMetadata(tileset, tileSetMissing)

	meta := &model.MapMetadata{
		ReplayPath:     path,
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
		// Some replays carry a few extra tile slots past the declared map size; trim to match header.
		meta.Tiles = meta.Tiles[:expected]
	} else if got < expected {
		return nil, fmt.Errorf("replay tile grid mismatch for %q: got %d tiles, want %d (%dx%d)", path, got, expected, meta.WidthTiles, meta.HeightTiles)
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
