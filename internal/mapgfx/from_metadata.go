package mapgfx

import (
	"errors"

	"github.com/marianogappa/scmapanalyzer/internal/model"
)

// MapDataFromMetadata builds render input from replay-derived map metadata.
func MapDataFromMetadata(meta *model.MapMetadata) (MapData, error) {
	if meta == nil {
		return MapData{}, errors.New("nil metadata")
	}
	if _, err := TilesetAssetFolderFromReplay(meta.TilesetKey); err != nil {
		return MapData{}, err
	}
	md := MapData{
		TileSet: meta.TilesetKey,
		Width:   meta.WidthTiles,
		Height:  meta.HeightTiles,
		Tiles:   meta.Tiles,
	}
	for _, m := range meta.MineralFields {
		md.Minerals = append(md.Minerals, Point{X: m.X, Y: m.Y})
	}
	for _, g := range meta.Geysers {
		md.Geysers = append(md.Geysers, Point{X: g.X, Y: g.Y})
	}
	return md, nil
}
