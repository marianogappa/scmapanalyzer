package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/marianogappa/scmapanalyzer/tools/hdbake/casc"
)

// resourceShadows are the SD shadow images minerals and geysers attach with imgul (0, 0).
var resourceShadows = []string{"neutral/min01Sha.grp", "neutral/min02Sha.grp", "neutral/min03Sha.grp", "neutral/geyShad.grp"}

// bakeResourceShadows writes frame 0 of each resource shadow grp as an SPR1 sprite (see
// internal/mapgfx/sprite) to <dir>/neutral__<name>.spr. Only opacity matters for a shadow.
func bakeResourceShadows(st *casc.Storage, dir string) error {
	for _, name := range resourceShadows {
		grp, err := st.ReadFile("unit/" + name)
		if err != nil {
			return err
		}
		w, h := int(binary.LittleEndian.Uint16(grp[2:])), int(binary.LittleEndian.Uint16(grp[4:]))
		spr := make([]byte, 8+w*h*2)
		copy(spr, "SPR1")
		binary.LittleEndian.PutUint16(spr[4:], uint16(w))
		binary.LittleEndian.PutUint16(spr[6:], uint16(h))
		decodeGrpFrame(grp, 0, func(x, y int, idx byte) {
			if x < w && y < h {
				spr[8+(y*w+x)*2] = idx
				spr[8+(y*w+x)*2+1] = 1
			}
		})
		out := strings.ToLower(strings.ReplaceAll(strings.TrimSuffix(name, ".grp"), "/", "__")) + ".spr"
		if err := os.WriteFile(filepath.Join(dir, out), spr, 0o644); err != nil {
			return err
		}
		fmt.Printf("resource shadow %s: %dx%d\n", out, w, h)
	}
	return nil
}
