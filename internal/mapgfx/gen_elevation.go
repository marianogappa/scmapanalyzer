//go:build ignore

// gen_elevation derives data/tilesets/<name>/elevation.bin from the original
// StarCraft cv5 + vf4 tileset files: one nibble per minitile (same tileID*16+mini
// indexing as features.bin) holding the ground level, 0 = low, 1 = mid, 2 = high.
// It is a one-time provenance tool; the proprietary inputs are not committed.
// Run from internal/mapgfx:
//
//	go run gen_elevation.go -src "/path/to/bwdata/tileset"
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

const (
	cv5EntrySize         = 52
	cv5MegaTileRefOffset = 0x14
	vf4Mid               = 0x0002
	vf4High              = 0x0004
)

var tilesets = []string{
	"ashworld", "badlands", "desert", "ice",
	"install", "jungle", "platform", "twilight",
}

func main() {
	src := flag.String("src", "", "directory holding <tileset>.cv5 and <tileset>.vf4")
	flag.Parse()
	for _, name := range tilesets {
		if err := convert(*src, name); err != nil {
			fmt.Fprintf(os.Stderr, "tileset %s: %v\n", name, err)
			os.Exit(1)
		}
	}
}

func convert(src, name string) error {
	cv5, err := os.ReadFile(filepath.Join(src, name+".cv5"))
	if err != nil {
		return err
	}
	vf4, err := os.ReadFile(filepath.Join(src, name+".vf4"))
	if err != nil {
		return err
	}
	tileCount := len(cv5) / cv5EntrySize * 16
	packed := make([]byte, (tileCount*16+1)/2)
	for tileID := 0; tileID < tileCount; tileID++ {
		off := (tileID>>4)*cv5EntrySize + cv5MegaTileRefOffset + (tileID&0x0F)*2
		mega := int(binary.LittleEndian.Uint16(cv5[off : off+2]))
		for mini := 0; mini < 16; mini++ {
			fo := mega*32 + mini*2
			if fo+2 > len(vf4) {
				continue
			}
			flags := binary.LittleEndian.Uint16(vf4[fo : fo+2])
			level := byte(0)
			switch {
			case flags&vf4High != 0:
				level = 2
			case flags&vf4Mid != 0:
				level = 1
			}
			nib := tileID*16 + mini
			if nib%2 == 0 {
				packed[nib/2] |= level
			} else {
				packed[nib/2] |= level << 4
			}
		}
	}
	out := make([]byte, 8, 8+len(packed))
	copy(out, "SME1")
	binary.LittleEndian.PutUint32(out[4:8], uint32(tileCount))
	out = append(out, packed...)
	dst := filepath.Join("data", "tilesets", name, "elevation.bin")
	if err := os.WriteFile(dst, out, 0o644); err != nil {
		return err
	}
	fmt.Printf("%-9s tiles=%d bytes=%d\n", name, tileCount, len(out))
	return nil
}
