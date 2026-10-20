package mapgfx

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"path"

	"github.com/marianogappa/scmapanalyzer/internal/mapgfx/tileset"
)

// Creep tiles are cv5 group 1 (tile ids 16..31). The game fills creep with slots 0..5 96% of the
// time and with the "feature" slots 6..12 (eggs, holes) otherwise; the texture uses plain slots
// only so its 128 px repeat does not show.
const (
	creepTileGroup = 1
	creepTexTiles  = 4
)

// creepTextureSlots is a fixed 4x4 arrangement of the plain creep slots with no equal neighbors,
// wrapping included, so the texture tiles seamlessly.
var creepTextureSlots = [creepTexTiles * creepTexTiles]int{
	0, 3, 1, 4,
	2, 5, 0, 3,
	4, 1, 2, 5,
	3, 0, 4, 1,
}

// CreepTexture renders a seamless 128x128 creep texture (32 px per tile) for a tileset.
func CreepTexture(tilesetKey string) (*image.Paletted, error) {
	pack, err := loadPack(tilesetKey)
	if err != nil {
		return nil, err
	}
	tiles := make([]uint16, len(creepTextureSlots))
	for i, slot := range creepTextureSlots {
		tiles[i] = uint16(creepTileGroup*16 + slot)
	}
	return tileset.RenderPackToPaletted(pack, creepTexTiles, creepTexTiles, tiles)
}

// CreepEdges returns the tileset's creep edge frames: frame k of [CreepEdgeFrame] (1-based) is the
// 32x32 cell at x = 32*(k-1). The game draws the frame over a tile without creep.
func CreepEdges(tilesetKey string) (*image.NRGBA, error) {
	folder, err := TilesetAssetFolderFromReplay(tilesetKey)
	if err != nil {
		return nil, err
	}
	b, err := assets.ReadFile(path.Join("data", "tilesets", folder, "creep_edges.png"))
	if err != nil {
		return nil, err
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	nrgba, ok := img.(*image.NRGBA)
	if !ok {
		return nil, fmt.Errorf("creep edges for %s are not NRGBA", folder)
	}
	return nrgba, nil
}

// CreepEdgeFrame returns the creep edge frame the game draws on a tile without creep, or 0 for
// none. Bits of neighbors, set when that neighbor has creep: 0 SE, 1 S, 2 SW, 3 E, 4 W, 5 NE,
// 6 N, 7 NW. The table
// is OpenBW's (ui.h, load_image_data), which reproduces the original game.
func CreepEdgeFrame(neighbors uint8) int {
	return creepEdgeFrames[neighbors]
}

var creepEdgeFrames = func() (frames [256]int) {
	var code [256]int
	for i := range 256 {
		v := 0
		if i&2 != 0 {
			v |= 0x10
		}
		if i&8 != 0 {
			v |= 0x24
		}
		if i&0x10 != 0 {
			v |= 9
		}
		if i&0x40 != 0 {
			v |= 2
		}
		if i&0xc0 == 0xc0 {
			v |= 1
		}
		if i&0x60 == 0x60 {
			v |= 4
		}
		if i&3 == 3 {
			v |= 0x20
		}
		if i&6 == 6 {
			v |= 8
		}
		if v&0x21 == 0x21 || v&0xc == 0xc {
			v |= 0x40
		}
		code[i] = v
	}
	var used [128]bool
	for _, v := range code {
		used[v] = true
	}
	var rank [128]int
	n := 0
	for v := range 128 {
		if used[v] {
			rank[v] = n
			n++
		}
	}
	for i, v := range code {
		frames[i] = rank[v]
	}
	return frames
}()

func loadPack(tilesetKey string) (*tileset.Pack, error) {
	folder, err := TilesetAssetFolderFromReplay(tilesetKey)
	if err != nil {
		return nil, err
	}
	return tileset.LoadPackFromFS(assets, folder)
}
