package main

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"path/filepath"

	"github.com/marianogappa/scmapanalyzer/tools/hdbake/casc"
)

var tilesets = []string{"ashworld", "badlands", "desert", "ice", "install", "jungle", "platform", "twilight"}

// bakeCreepEdges renders each tileset's creep edge grp (tileset/<name>.grp, palette-indexed) into
// <dir>/<name>/creep_edges.png: frame k (1-based, as the game numbers them) at x = 32*(k-1).
func bakeCreepEdges(st *casc.Storage, dir string) error {
	for _, ts := range tilesets {
		grp, err := st.ReadFile("tileset/" + ts + ".grp")
		if err != nil {
			return err
		}
		wpe, err := st.ReadFile("tileset/" + ts + ".wpe")
		if err != nil {
			return err
		}
		atlas, err := decodeCreepGrp(grp, wpe)
		if err != nil {
			return fmt.Errorf("%s: %w", ts, err)
		}
		if err := writePNG(filepath.Join(dir, ts, "creep_edges.png"), atlas); err != nil {
			return err
		}
		fmt.Printf("creep edges %s: %d frames\n", ts, atlas.Bounds().Dx()/32)
	}
	return nil
}

func decodeCreepGrp(grp, wpe []byte) (*image.NRGBA, error) {
	n := int(binary.LittleEndian.Uint16(grp))
	if w, h := binary.LittleEndian.Uint16(grp[2:]), binary.LittleEndian.Uint16(grp[4:]); w != 32 || h != 32 {
		return nil, fmt.Errorf("creep grp is %dx%d, want 32x32", w, h)
	}
	atlas := image.NewNRGBA(image.Rect(0, 0, 32*n, 32))
	for f := range n {
		h := grp[6+f*8:]
		fx, fy, fw, fh := int(h[0]), int(h[1]), int(h[2]), int(h[3])
		off := int(binary.LittleEndian.Uint32(h[4:]))
		set := func(x, y int, idx byte) {
			atlas.SetNRGBA(32*f+fx+x, fy+y, color.NRGBA{wpe[int(idx)*4], wpe[int(idx)*4+1], wpe[int(idx)*4+2], 255})
		}
		for y := range fh {
			p := off + int(binary.LittleEndian.Uint16(grp[off+y*2:]))
			for x := 0; x < fw; {
				c := int(grp[p])
				p++
				switch {
				case c&0x80 != 0:
					x += c & 0x7f
				case c&0x40 != 0:
					for range c & 0x3f {
						set(x, y, grp[p])
						x++
					}
					p++
				default:
					for range c {
						set(x, y, grp[p])
						p++
						x++
					}
				}
			}
		}
	}
	return atlas, nil
}
