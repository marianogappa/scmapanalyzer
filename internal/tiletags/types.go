package tiletags

type TileSetTags struct {
	TileSetID   int      `json:"tileset_id"`
	TileSetName string   `json:"tileset_name"`
	TileSetKey  string   `json:"tileset_key"`
	SourceMap   string   `json:"source_map"`
	WallTileIDs []uint16 `json:"wall_tile_ids"`
	RampTileIDs []uint16 `json:"ramp_tile_ids"`
}
