package model

type MapResource struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type StartLocation struct {
	X      int  `json:"x"`
	Y      int  `json:"y"`
	SlotID byte `json:"slot_id"`
}

type MapMetadata struct {
	ReplayPath string `json:"replay_path"`

	MapName        string `json:"map_name"`
	MapDataName    string `json:"map_data_name"`
	WidthTiles     int    `json:"width_tiles"`
	HeightTiles    int    `json:"height_tiles"`
	Tileset        int    `json:"tileset"`
	TilesetName    string `json:"tileset_name"`
	TilesetKey     string `json:"tileset_key"`
	TileSetMissing bool   `json:"tile_set_missing"`
	Tiles          []uint16
	MineralFields  []MapResource   `json:"-"`
	Geysers        []MapResource   `json:"-"`
	StartLocations []StartLocation `json:"-"`
}

type Base struct {
	CenterX       float64 `json:"center_x"`
	CenterY       float64 `json:"center_y"`
	NaturalRadius float64 `json:"natural_radius"`
	GeoRadius     float64 `json:"geo_radius"`
	StartCount    int     `json:"start_count"`
	IsStarting    bool    `json:"is_starting"`
	DisplayName   string  `json:"display_name"`
}

type MatchCandidate struct {
	ImagePath string  `json:"image_path"`
	ImageName string  `json:"image_name"`
	Score     float64 `json:"score"`
}

type MatchResult struct {
	Accepted   bool             `json:"accepted"`
	Reason     string           `json:"reason,omitempty"`
	ReplayMap  string           `json:"replay_map"`
	Chosen     *MatchCandidate  `json:"chosen,omitempty"`
	Candidates []MatchCandidate `json:"candidates,omitempty"`
}

type Summary struct {
	ReplayPath        string          `json:"replay_path"`
	MapName           string          `json:"map_name"`
	MapDataName       string          `json:"map_data_name"`
	MatchedImage      string          `json:"matched_image"`
	MatchScore        float64         `json:"match_score"`
	WidthTiles        int             `json:"width_tiles"`
	HeightTiles       int             `json:"height_tiles"`
	Tileset           int             `json:"tileset"`
	TilesetName       string          `json:"tileset_name"`
	TilesetKey        string          `json:"tileset_key"`
	TilesetGalleryDir string          `json:"tileset_gallery_dir"`
	MineralCount      int             `json:"mineral_count"`
	GeyserCount       int             `json:"geyser_count"`
	StartCount        int             `json:"start_count"`
	DetectedBases     []Base          `json:"detected_bases"`
	StartLocations    []StartLocation `json:"start_locations"`
}
