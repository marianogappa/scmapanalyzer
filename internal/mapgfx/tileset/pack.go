package tileset

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"io/fs"
	"path"
	"strings"
)

// Pack is a self-describing tileset graphics bundle that replaces the original
// StarCraft cv5/vx4ex/vr4/wpe files. It carries exactly the data the renderer
// needs and nothing else:
//
//   - Palette: 256 RGB colors (was WPE).
//   - TileMegatile: for map tile ID t, TileMegatile[t] is its megatile index
//     (was the cv5 megatile-reference table; index t == group*16 + slot).
//   - Megatiles: flattened 16-entry minitile references per megatile; each entry
//     is (minitileID<<1 | hFlip) (was the low 16 bits of a vx4ex entry).
//   - Minitiles: 8x8 palette-indexed pixels, 64 bytes each (was vr4).
//
// Binary layout (little-endian), produced by gen_pack.go:
//
//	magic        [4]byte  = "TSP1"
//	tileCount    uint32   // len(TileMegatile)
//	megatileN    uint32   // len(Megatiles)/16
//	minitileN    uint32   // len(Minitiles)/64
//	palette      [256]{R,G,B uint8}
//	tileMegatile [tileCount]uint16
//	megatiles    [megatileN*16]uint16
//	minitiles    [minitileN*64]uint8
const packMagic = "TSP1"

type Pack struct {
	Palette      color.Palette
	TileMegatile []uint16
	Megatiles    []uint16
	Minitiles    []byte
}

// LoadPackFromFS reads data/tilesets/<name>/<name>.tspk from fsys.
func LoadPackFromFS(fsys fs.FS, tilesetDir string) (*Pack, error) {
	t := strings.ToLower(tilesetDir)
	p := path.Join("data", "tilesets", t, t+".tspk")
	b, err := fs.ReadFile(fsys, p)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", p, err)
	}
	return DecodePack(b)
}

func DecodePack(b []byte) (*Pack, error) {
	if len(b) < 16 || string(b[0:4]) != packMagic {
		return nil, fmt.Errorf("invalid tileset pack header")
	}
	tileCount := int(binary.LittleEndian.Uint32(b[4:8]))
	megatileN := int(binary.LittleEndian.Uint32(b[8:12]))
	minitileN := int(binary.LittleEndian.Uint32(b[12:16]))

	off := 16
	palBytes := 256 * 3
	tmBytes := tileCount * 2
	mtBytes := megatileN * 16 * 2
	miniBytes := minitileN * 64
	if len(b) != off+palBytes+tmBytes+mtBytes+miniBytes {
		return nil, fmt.Errorf("tileset pack size mismatch: got %d", len(b))
	}

	pal := make(color.Palette, 256)
	for i := 0; i < 256; i++ {
		r := b[off+i*3+0]
		g := b[off+i*3+1]
		bl := b[off+i*3+2]
		pal[i] = color.RGBA{R: r, G: g, B: bl, A: 255}
	}
	off += palBytes

	tileMegatile := make([]uint16, tileCount)
	for i := range tileMegatile {
		tileMegatile[i] = binary.LittleEndian.Uint16(b[off+i*2 : off+i*2+2])
	}
	off += tmBytes

	megatiles := make([]uint16, megatileN*16)
	for i := range megatiles {
		megatiles[i] = binary.LittleEndian.Uint16(b[off+i*2 : off+i*2+2])
	}
	off += mtBytes

	minitiles := make([]byte, miniBytes)
	copy(minitiles, b[off:off+miniBytes])

	return &Pack{
		Palette:      pal,
		TileMegatile: tileMegatile,
		Megatiles:    megatiles,
		Minitiles:    minitiles,
	}, nil
}

// RenderPackToPaletted renders a map tile grid to a paletted image (32 px per tile).
// It reproduces the exact minitile lookup and horizontal-flip behavior of the
// original cv5/vx4ex/vr4 pipeline, including its out-of-range skips (which leave
// palette index 0).
func RenderPackToPaletted(p *Pack, widthTiles, heightTiles int, tiles []uint16) (*image.Paletted, error) {
	if len(tiles) != widthTiles*heightTiles {
		return nil, fmt.Errorf("tiles length mismatch: got %d expected %d", len(tiles), widthTiles*heightTiles)
	}
	imgW := widthTiles * 32
	imgH := heightTiles * 32
	img := image.NewPaletted(image.Rect(0, 0, imgW, imgH), p.Palette)

	for ty := 0; ty < heightTiles; ty++ {
		for tx := 0; tx < widthTiles; tx++ {
			tileID := int(tiles[ty*widthTiles+tx])
			if tileID >= len(p.TileMegatile) {
				continue
			}
			megatileRef := int(p.TileMegatile[tileID])
			mtBase := megatileRef * 16
			if mtBase+16 > len(p.Megatiles) {
				continue
			}
			for my := 0; my < 4; my++ {
				for mx := 0; mx < 4; mx++ {
					miniRef := p.Megatiles[mtBase+my*4+mx]
					miniID := int(miniRef >> 1)
					hflip := (miniRef & 1) != 0
					vrBase := miniID * 64
					if vrBase+64 > len(p.Minitiles) {
						continue
					}
					dstX0 := tx*32 + mx*8
					dstY0 := ty*32 + my*8
					for py := 0; py < 8; py++ {
						dst := img.PixOffset(dstX0, dstY0+py)
						src := vrBase + py*8
						if hflip {
							img.Pix[dst+0] = p.Minitiles[src+7]
							img.Pix[dst+1] = p.Minitiles[src+6]
							img.Pix[dst+2] = p.Minitiles[src+5]
							img.Pix[dst+3] = p.Minitiles[src+4]
							img.Pix[dst+4] = p.Minitiles[src+3]
							img.Pix[dst+5] = p.Minitiles[src+2]
							img.Pix[dst+6] = p.Minitiles[src+1]
							img.Pix[dst+7] = p.Minitiles[src+0]
						} else {
							copy(img.Pix[dst:dst+8], p.Minitiles[src:src+8])
						}
					}
				}
			}
		}
	}
	return img, nil
}
