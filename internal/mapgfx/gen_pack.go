//go:build ignore

// gen_pack converts the original StarCraft tileset (cv5/vx4ex/vr4/wpe) and
// sprite (grp) files into the repository's self-contained pack formats:
// <tileset>.tspk (see tileset/pack.go) and <sprite>.spr (see sprite/sprite.go).
//
// It is a one-time provenance/build tool and is excluded from normal builds.
// It requires the original proprietary StarCraft files to be present under
// data/tilesets/<name>/ and data/sprites/, which are NOT committed to this
// repository. Run from internal/mapgfx:
//
//	go run gen_pack.go
package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const (
	cv5EntrySize         = 52
	cv5MegaTileRefOffset = 0x14
)

var tilesets = []string{
	"ashworld", "badlands", "desert", "ice",
	"install", "jungle", "platform", "twilight",
}

func main() {
	for _, name := range tilesets {
		if err := convertTileset(name); err != nil {
			fmt.Fprintf(os.Stderr, "tileset %s: %v\n", name, err)
			os.Exit(1)
		}
	}
	if err := convertSprites(); err != nil {
		fmt.Fprintf(os.Stderr, "sprites: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("done")
}

func convertTileset(name string) error {
	dir := filepath.Join("data", "tilesets", name)
	cv5, err := os.ReadFile(filepath.Join(dir, name+".cv5"))
	if err != nil {
		return err
	}
	vx4, err := os.ReadFile(filepath.Join(dir, name+".vx4ex"))
	if err != nil {
		return err
	}
	vr4, err := os.ReadFile(filepath.Join(dir, name+".vr4"))
	if err != nil {
		return err
	}
	wpe, err := os.ReadFile(filepath.Join(dir, name+".wpe"))
	if err != nil {
		return err
	}
	if len(wpe) != 1024 {
		return fmt.Errorf("unexpected wpe size %d", len(wpe))
	}
	if len(vx4)%64 != 0 {
		return fmt.Errorf("vx4ex not a multiple of 64: %d", len(vx4))
	}
	if len(vr4)%64 != 0 {
		return fmt.Errorf("vr4 not a multiple of 64: %d", len(vr4))
	}

	// tileMegatile[tileID] mirrors the renderer's cv5 lookup; tileID == group*16+slot.
	var tileMegatile []uint16
	for tileID := 0; ; tileID++ {
		group := tileID >> 4
		slot := tileID & 0x0F
		mtOff := group*cv5EntrySize + cv5MegaTileRefOffset + slot*2
		if mtOff+2 > len(cv5) {
			break
		}
		tileMegatile = append(tileMegatile, binary.LittleEndian.Uint16(cv5[mtOff:mtOff+2]))
	}

	megatileN := len(vx4) / 64
	megatiles := make([]uint16, megatileN*16)
	for i := range megatiles {
		ref32 := binary.LittleEndian.Uint32(vx4[i*4 : i*4+4])
		megatiles[i] = uint16(ref32 & 0xFFFF)
	}

	minitileN := len(vr4) / 64

	buf := make([]byte, 0, 16+256*3+len(tileMegatile)*2+len(megatiles)*2+len(vr4))
	buf = append(buf, "TSP1"...)
	buf = appendU32(buf, uint32(len(tileMegatile)))
	buf = appendU32(buf, uint32(megatileN))
	buf = appendU32(buf, uint32(minitileN))
	for i := 0; i < 256; i++ {
		buf = append(buf, wpe[i*4+0], wpe[i*4+1], wpe[i*4+2])
	}
	for _, v := range tileMegatile {
		buf = appendU16(buf, v)
	}
	for _, v := range megatiles {
		buf = appendU16(buf, v)
	}
	buf = append(buf, vr4...)

	out := filepath.Join(dir, name+".tspk")
	if err := os.WriteFile(out, buf, 0o644); err != nil {
		return err
	}
	fmt.Printf("%-9s tspk=%d (was cv5+vx4ex+vr4+wpe+vf4)\n", name, len(buf))
	return nil
}

func convertSprites() error {
	dir := filepath.Join("data", "sprites")
	b, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return err
	}
	var m struct {
		Sprites map[string]string `json:"sprites"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}

	converted := map[string]string{} // old grp filename -> new spr filename
	for _, rel := range m.Sprites {
		if _, done := converted[rel]; done {
			continue
		}
		grpData, err := os.ReadFile(filepath.Join(dir, rel))
		if err != nil {
			return err
		}
		spr, err := grpFirstFrameToSprite(grpData)
		if err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		newName := rel[:len(rel)-len(filepath.Ext(rel))] + ".spr"
		if err := os.WriteFile(filepath.Join(dir, newName), spr, 0o644); err != nil {
			return err
		}
		converted[rel] = newName
	}

	newSprites := map[string]string{}
	for k, rel := range m.Sprites {
		newSprites[k] = converted[rel]
	}
	nm, err := json.MarshalIndent(struct {
		Sprites map[string]string `json:"sprites"`
	}{Sprites: newSprites}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), append(nm, '\n'), 0o644); err != nil {
		return err
	}

	keys := make([]string, 0, len(converted))
	for k := range converted {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("sprite %s -> %s\n", k, converted[k])
	}
	return nil
}

// grpFirstFrameToSprite decodes frame 0 of a GRP into the SPR1 format,
// recording palette indices + opacity exactly as the old GRP RGBA decoder
// painted them (last write wins, same bounds).
func grpFirstFrameToSprite(data []byte) ([]byte, error) {
	if len(data) < 6 {
		return nil, fmt.Errorf("invalid grp size %d", len(data))
	}
	fc := int(binary.LittleEndian.Uint16(data[0:2]))
	cw := int(binary.LittleEndian.Uint16(data[2:4]))
	ch := int(binary.LittleEndian.Uint16(data[4:6]))
	if fc <= 0 || len(data) < 6+fc*8 {
		return nil, fmt.Errorf("invalid grp frame table")
	}
	base := 6
	xOff := int(data[base+0])
	yOff := int(data[base+1])
	fw := int(data[base+2])
	fh := int(data[base+3])
	frameOff := int(binary.LittleEndian.Uint32(data[base+4 : base+8]))
	if frameOff <= 0 || frameOff >= len(data) {
		return nil, fmt.Errorf("first frame offset out of range: %d", frameOff)
	}

	idx := make([]byte, cw*ch)
	op := make([]byte, cw*ch)
	set := func(px, py int, v byte) {
		if px >= 0 && py >= 0 && px < cw && py < ch {
			idx[py*cw+px] = v
			op[py*cw+px] = 1
		}
	}

	if fw > 0 && fh > 0 {
		lineTableEnd := frameOff + fh*2
		if lineTableEnd > len(data) {
			return nil, fmt.Errorf("frame line table truncated")
		}
		for y := 0; y < fh; y++ {
			rowRel := int(binary.LittleEndian.Uint16(data[frameOff+y*2 : frameOff+y*2+2]))
			ptr := frameOff + rowRel
			x := 0
			for x < fw && ptr < len(data) {
				opc := data[ptr]
				ptr++
				switch {
				case opc >= 0x80:
					x += int(opc - 0x80)
				case opc >= 0x40:
					run := int(opc - 0x40)
					if ptr >= len(data) {
						break
					}
					v := data[ptr]
					ptr++
					for i := 0; i < run && x < fw; i++ {
						set(xOff+x, yOff+y, v)
						x++
					}
				case opc > 0:
					run := int(opc)
					for i := 0; i < run && x < fw && ptr < len(data); i++ {
						v := data[ptr]
						ptr++
						set(xOff+x, yOff+y, v)
						x++
					}
				default:
					x = fw
				}
			}
		}
	}

	buf := make([]byte, 0, 8+cw*ch*2)
	buf = append(buf, "SPR1"...)
	buf = appendU16(buf, uint16(cw))
	buf = appendU16(buf, uint16(ch))
	for i := 0; i < cw*ch; i++ {
		buf = append(buf, idx[i], op[i])
	}
	return buf, nil
}

func appendU16(b []byte, v uint16) []byte {
	return append(b, byte(v), byte(v>>8))
}

func appendU32(b []byte, v uint32) []byte {
	return append(b, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}
