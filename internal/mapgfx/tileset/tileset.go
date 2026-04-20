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

const (
	cv5EntrySize         = 52
	cv5MegaTileRefOffset = 0x14
)

type Raw struct {
	Name  string
	CV5   []byte
	VF4   []byte
	VX4EX []byte
	VR4   []byte
	WPE   []byte
}

// LoadFromFS reads CV5/VF4/VX4EX/VR4/WPE for a Blizzard tileset folder name
// (e.g. "badlands", "platform") from fsys rooted paths like data/tilesets/<name>/.
func LoadFromFS(fsys fs.FS, tilesetDir string) (*Raw, error) {
	t := strings.ToLower(tilesetDir)
	base := path.Join("data", "tilesets", t)
	read := func(name string) ([]byte, error) {
		b, err := fs.ReadFile(fsys, path.Join(base, name))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		return b, nil
	}
	cv5, err := read(t + ".cv5")
	if err != nil {
		return nil, err
	}
	vf4, err := read(t + ".vf4")
	if err != nil {
		return nil, err
	}
	vx4, err := read(t + ".vx4ex")
	if err != nil {
		return nil, err
	}
	vr4, err := read(t + ".vr4")
	if err != nil {
		return nil, err
	}
	wpe, err := read(t + ".wpe")
	if err != nil {
		return nil, err
	}
	return &Raw{Name: t, CV5: cv5, VF4: vf4, VX4EX: vx4, VR4: vr4, WPE: wpe}, nil
}

func PaletteFromWPE(wpe []byte) (color.Palette, error) {
	if len(wpe) != 1024 {
		return nil, fmt.Errorf("unexpected wpe size: %d", len(wpe))
	}
	pal := make(color.Palette, 256)
	for i := 0; i < 256; i++ {
		r := wpe[i*4+0]
		g := wpe[i*4+1]
		b := wpe[i*4+2]
		pal[i] = color.RGBA{R: r, G: g, B: b, A: 255}
	}
	return pal, nil
}

func TileCount(cv5 []byte) int {
	return (len(cv5) / cv5EntrySize) * 16
}

func RenderMapToPaletted(raw *Raw, widthTiles, heightTiles int, tiles []uint16) (*image.Paletted, error) {
	if len(tiles) != widthTiles*heightTiles {
		return nil, fmt.Errorf("tiles length mismatch: got %d expected %d", len(tiles), widthTiles*heightTiles)
	}
	pal, err := PaletteFromWPE(raw.WPE)
	if err != nil {
		return nil, err
	}
	imgW := widthTiles * 32
	imgH := heightTiles * 32
	img := image.NewPaletted(image.Rect(0, 0, imgW, imgH), pal)

	for ty := 0; ty < heightTiles; ty++ {
		for tx := 0; tx < widthTiles; tx++ {
			tileID := tiles[ty*widthTiles+tx]
			group := int(tileID >> 4)
			slot := int(tileID & 0x0F)
			cv5Base := group * cv5EntrySize
			mtOff := cv5Base + cv5MegaTileRefOffset + slot*2
			if mtOff+2 > len(raw.CV5) {
				continue
			}
			megatileRef := int(binary.LittleEndian.Uint16(raw.CV5[mtOff : mtOff+2]))
			vxBase := megatileRef * 64 // VX4EX
			if vxBase+64 > len(raw.VX4EX) {
				continue
			}
			for my := 0; my < 4; my++ {
				for mx := 0; mx < 4; mx++ {
					ref32 := binary.LittleEndian.Uint32(raw.VX4EX[vxBase+(my*4+mx)*4 : vxBase+(my*4+mx)*4+4])
					miniRef := uint16(ref32 & 0xFFFF)
					miniID := int(miniRef >> 1)
					hflip := (miniRef & 1) != 0
					vrBase := miniID * 64
					if vrBase+64 > len(raw.VR4) {
						continue
					}
					dstX0 := tx*32 + mx*8
					dstY0 := ty*32 + my*8
					for py := 0; py < 8; py++ {
						dst := img.PixOffset(dstX0, dstY0+py)
						src := vrBase + py*8
						if hflip {
							img.Pix[dst+0] = raw.VR4[src+7]
							img.Pix[dst+1] = raw.VR4[src+6]
							img.Pix[dst+2] = raw.VR4[src+5]
							img.Pix[dst+3] = raw.VR4[src+4]
							img.Pix[dst+4] = raw.VR4[src+3]
							img.Pix[dst+5] = raw.VR4[src+2]
							img.Pix[dst+6] = raw.VR4[src+1]
							img.Pix[dst+7] = raw.VR4[src+0]
						} else {
							copy(img.Pix[dst:dst+8], raw.VR4[src:src+8])
						}
					}
				}
			}
		}
	}
	return img, nil
}
