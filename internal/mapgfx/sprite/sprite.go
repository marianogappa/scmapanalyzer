package sprite

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
)

// Sprite is a palette-indexed still frame that replaces the original StarCraft
// GRP sprite files. Frame 0 of each GRP is pre-decoded into a fixed w*h canvas
// of palette indices plus an opacity mask; the tileset palette is applied at
// render time (the same index maps to different colors per tileset), exactly as
// the GRP decoder did.
//
// Binary layout (little-endian), produced by gen_pack.go:
//
//	magic  [4]byte = "SPR1"
//	width  uint16
//	height uint16
//	pixels [width*height]{index, opaque uint8}  // opaque: 0 transparent, 1 painted
const spriteMagic = "SPR1"

// DecodeRGBA rebuilds the RGBA frame by coloring painted pixels with palette.
func DecodeRGBA(data []byte, palette color.Palette) (*image.RGBA, error) {
	if len(data) < 8 || string(data[0:4]) != spriteMagic {
		return nil, fmt.Errorf("invalid sprite header")
	}
	w := int(binary.LittleEndian.Uint16(data[4:6]))
	h := int(binary.LittleEndian.Uint16(data[6:8]))
	body := data[8:]
	if len(body) != w*h*2 {
		return nil, fmt.Errorf("sprite payload size mismatch: got %d expected %d", len(body), w*h*2)
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 2
			if body[i+1] == 0 {
				continue
			}
			img.Set(x, y, palette[body[i]])
		}
	}
	return img, nil
}
