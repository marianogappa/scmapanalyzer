package mapgfx

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

const (
	FeatureWalkable   uint8 = 1 << 0
	FeatureRamp       uint8 = 1 << 1
	FeatureBlocksView uint8 = 1 << 2
	FeatureBuildable  uint8 = 1 << 3
)

// MiniTileGrid is row-major minitile feature masks (4 bits each) at 4× map resolution.
type MiniTileGrid struct {
	Width  int
	Height int
	Data   []uint8 // each cell: feature nibble (see Feature* constants)
}

func (g *MiniTileGrid) FeatureAt(x, y int) uint8 {
	if x < 0 || y < 0 || x >= g.Width || y >= g.Height {
		return 0
	}
	return g.Data[y*g.Width+x] & 0x0F
}

type tileFeatures struct {
	tileCount int
	packed    []byte
}

func (t *tileFeatures) get(nibbleIndex int) uint8 {
	b := t.packed[nibbleIndex/2]
	if nibbleIndex%2 == 0 {
		return b & 0x0F
	}
	return (b >> 4) & 0x0F
}

func loadTileFeatures(fsys fs.FS, tilesetFolder string) (*tileFeatures, error) {
	t := strings.ToLower(tilesetFolder)
	p := path.Join("data", "tilesets", t, "features.bin")
	b, err := fs.ReadFile(fsys, p)
	if err != nil {
		return nil, fmt.Errorf("read features %s: %w", p, err)
	}
	if len(b) < 8 || string(b[0:4]) != "SMF1" {
		return nil, errors.New("invalid features file")
	}
	tileCount := int(binary.LittleEndian.Uint32(b[4:8]))
	payload := b[8:]
	exp := (tileCount*16 + 1) / 2
	if len(payload) != exp {
		return nil, fmt.Errorf("features payload size mismatch: got %d expected %d", len(payload), exp)
	}
	return &tileFeatures{tileCount: tileCount, packed: payload}, nil
}

// BuildMiniTileGrid expands map tile IDs into a minitile grid using embedded features.bin.
func BuildMiniTileGrid(tilesetFolder string, width, height int, tiles []uint16) (*MiniTileGrid, error) {
	if width <= 0 || height <= 0 {
		return nil, errors.New("invalid map dimensions")
	}
	if len(tiles) != width*height {
		return nil, errors.New("tile grid mismatch")
	}
	tf, err := loadTileFeatures(assets, tilesetFolder)
	if err != nil {
		return nil, err
	}
	gridW := width * 4
	gridH := height * 4
	data := make([]uint8, gridW*gridH)

	for ty := 0; ty < height; ty++ {
		for tx := 0; tx < width; tx++ {
			tileID := int(tiles[ty*width+tx])
			if tileID < 0 || tileID >= tf.tileCount {
				continue
			}
			baseNib := tileID * 16
			for my := 0; my < 4; my++ {
				for mx := 0; mx < 4; mx++ {
					v := tf.get(baseNib + my*4 + mx)
					data[(ty*4+my)*gridW+(tx*4+mx)] = v
				}
			}
		}
	}
	return &MiniTileGrid{Width: gridW, Height: gridH, Data: data}, nil
}
