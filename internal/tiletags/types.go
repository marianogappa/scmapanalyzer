package tiletags

type TileSetTags struct {
	TileSetID   int      `json:"tileset_id"`
	TileSetName string   `json:"tileset_name"`
	TileSetKey  string   `json:"tileset_key"`
	WallTileIDs []uint16 `json:"wall_tile_ids"`
	RampTileIDs []uint16 `json:"ramp_tile_ids"`
	// WalkableIDs is only used transiently while merging tags.
	// It intentionally stays out of serialized output.
	WalkableIDs []uint16 `json:"-"`
}
