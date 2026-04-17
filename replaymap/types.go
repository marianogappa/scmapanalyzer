package replaymap

import "github.com/marianogappa/scmapanalyzer/internal/model"

type TilePoint struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type BasePolygon struct {
	Name             string      `json:"name"`
	Kind             string      `json:"kind"`
	Clock            int         `json:"clock"`
	CenterTile       TilePoint   `json:"center_tile"`
	PolygonVertices  []TilePoint `json:"polygon_vertices"`
	NaturalExpansion string      `json:"natural_expansion,omitempty"`
}

type Result struct {
	ReplayPath string        `json:"replay_path"`
	MapName    string        `json:"map_name"`
	TileSetKey string        `json:"tileset_key"`
	Starts     []BasePolygon `json:"starting_locations"`
	Expas      []BasePolygon `json:"expansions"`
}

type DebugData struct {
	WidthTiles   int
	HeightTiles  int
	StartMasks   [][]bool
	ExpaMasks    [][]bool
	NaturalLinks []NaturalLink
	WallMask     []bool
	RampMask     []bool
}

type AnalyzeOutput struct {
	Result *Result
	Debug  *DebugData
	Bases  []model.Base
}

type NaturalLink struct {
	StartIndex int         `json:"start_index"`
	ExpaIndex  int         `json:"expa_index"`
	Path       []TilePoint `json:"path"`
}
