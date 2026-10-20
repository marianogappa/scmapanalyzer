package scmapanalyzer

import "github.com/marianogappa/scmapanalyzer/internal/mapgfx"

// CreepTexturePNG returns a seamless 128x128 PNG of a tileset's creep (4x4 tiles at 32 px per
// tile), for filling creep areas. Tileset is a folder name ("badlands") or a replay tileset key,
// as in map images.
func CreepTexturePNG(tileset string) ([]byte, error) {
	img, err := mapgfx.CreepTexture(tileset)
	if err != nil {
		return nil, err
	}
	return encodeUnitPNG(img)
}

// CreepNeighbors flags which of a tile's 8 neighbors have creep, for [CreepEdgeFrame].
type CreepNeighbors uint8

const (
	CreepSE CreepNeighbors = 1 << iota
	CreepS
	CreepSW
	CreepE
	CreepW
	CreepNE
	CreepN
	CreepNW
)

// CreepEdgeFrame returns the creep edge frame the game draws over a tile without creep, given
// which neighbors have creep, or 0 for none. Frames index [CreepEdgesPNG].
func CreepEdgeFrame(neighbors CreepNeighbors) int {
	return mapgfx.CreepEdgeFrame(uint8(neighbors))
}

// CreepEdgesPNG returns a tileset's creep edge frames as one 32 px tall PNG: frame k of
// [CreepEdgeFrame] is the 32x32 cell at x = 32*(k-1), transparent where it shows the tile below.
func CreepEdgesPNG(tileset string) ([]byte, error) {
	img, err := mapgfx.CreepEdges(tileset)
	if err != nil {
		return nil, err
	}
	return encodeUnitPNG(img)
}
