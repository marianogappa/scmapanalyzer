package replaymap

import "github.com/marianogappa/scmapanalyzer/internal/model"

// TilePoint uses minitile coordinates (8×8 px steps); map size is 4× map-tile width/height.
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
	MineralOnly      bool        `json:"mineral_only"`
	NaturalExpansion string      `json:"natural_expansion,omitempty"`
}

type Result struct {
	MapName    string        `json:"map_name"`
	TileSetKey string        `json:"tileset_key"`
	Starts     []BasePolygon `json:"starting_locations"`
	Expas      []BasePolygon `json:"expansions"`
}

type DebugData struct {
	WidthMinitiles  int
	HeightMinitiles int
	StartMasks      [][]bool
	ExpaMasks       [][]bool
	NaturalLinks    []NaturalLink
	WallMask        []bool
	RampMask        []bool
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
