package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

const (
	unitCount   = 228
	flingyCount = 209
	spriteCount = 517
	imageCount  = 999

	drawFuncShadow = 10
)

type imageType struct {
	grp      int
	drawFunc int
	iscript  int
	special  int
}

type gameData struct {
	unitFlingy   []int
	unitSubunit  []int
	flingySprite []int
	spriteImage  []int
	images       []imageType
	imagesTbl    []string
	iscript      []byte
	read         func(name string) ([]byte, error)
}

func loadGameData(read func(name string) ([]byte, error)) (*gameData, error) {
	d := &gameData{read: read}
	files := map[string][]byte{}
	for _, name := range []string{"arr/units.dat", "arr/flingy.dat", "arr/sprites.dat", "arr/images.dat", "arr/images.tbl", "scripts/iscript.bin"} {
		b, err := read(name)
		if err != nil {
			return nil, err
		}
		files[name] = b
	}
	units := files["arr/units.dat"]
	for i := range unitCount {
		d.unitFlingy = append(d.unitFlingy, int(units[i]))
		d.unitSubunit = append(d.unitSubunit, int(binary.LittleEndian.Uint16(units[unitCount+i*2:])))
	}
	for i := range flingyCount {
		d.flingySprite = append(d.flingySprite, int(binary.LittleEndian.Uint16(files["arr/flingy.dat"][i*2:])))
	}
	for i := range spriteCount {
		d.spriteImage = append(d.spriteImage, int(binary.LittleEndian.Uint16(files["arr/sprites.dat"][i*2:])))
	}
	if err := d.parseImagesDat(files["arr/images.dat"]); err != nil {
		return nil, err
	}
	tbl := files["arr/images.tbl"]
	d.imagesTbl = []string{""}
	for i := range int(binary.LittleEndian.Uint16(tbl)) {
		start := int(binary.LittleEndian.Uint16(tbl[2+i*2:]))
		end := start
		for tbl[end] != 0 {
			end++
		}
		d.imagesTbl = append(d.imagesTbl, string(tbl[start:end]))
	}
	d.iscript = files["scripts/iscript.bin"]
	return d, nil
}

func (d *gameData) parseImagesDat(b []byte) error {
	if len(b) != imageCount*(4*8+6) {
		return fmt.Errorf("images.dat: unexpected size %d", len(b))
	}
	off := 0
	u32 := func() []int {
		r := make([]int, imageCount)
		for i := range r {
			r[i] = int(binary.LittleEndian.Uint32(b[off+i*4:]))
		}
		off += imageCount * 4
		return r
	}
	u8 := func() []int {
		r := make([]int, imageCount)
		for i := range r {
			r[i] = int(b[off+i])
		}
		off += imageCount
		return r
	}
	grp := u32()
	u8() // turns
	u8() // clickable
	u8() // full iscript
	u8() // draw if cloaked
	drawFunc := u8()
	u8() // remapping
	iscript := u32()
	u32() // shield overlay
	u32() // attack overlay
	u32() // damage overlay
	special := u32()
	for i := range imageCount {
		d.images = append(d.images, imageType{grp: grp[i], drawFunc: drawFunc[i], iscript: iscript[i], special: special[i]})
	}
	return nil
}

func (d *gameData) unitImage(unit int) int {
	return d.spriteImage[d.flingySprite[d.unitFlingy[unit]]]
}

// loOffset returns an image's "special" LO offset for a frame, in SD pixels. Turrets attach there.
func (d *gameData) loOffset(imageID, frame int) (int, int, error) {
	idx := d.images[imageID].special
	if idx <= 0 || idx >= len(d.imagesTbl) {
		return 0, 0, fmt.Errorf("image %d has no special LO file", imageID)
	}
	b, err := d.read("unit/" + strings.ReplaceAll(d.imagesTbl[idx], `\`, "/"))
	if err != nil {
		return 0, 0, err
	}
	frames := int(binary.LittleEndian.Uint32(b))
	if frame >= frames {
		return 0, 0, fmt.Errorf("LO file of image %d has %d frames, want %d", imageID, frames, frame)
	}
	off := int(binary.LittleEndian.Uint32(b[8+frame*4:]))
	return int(int8(b[off])), int(int8(b[off+1])), nil
}

// shadowOf finds the shadow image the unit's init animation attaches with imgul, with its offset in SD pixels.
func (d *gameData) shadowOf(imageID int) (shadow, dx, dy int, ok bool) {
	addr, err := d.initAnimation(d.images[imageID].iscript)
	if err != nil {
		return 0, 0, 0, false
	}
	for _, op := range d.disassemble(addr, 32) {
		if op.name != "imgul" {
			continue
		}
		if s := op.args[0]; d.images[s].drawFunc == drawFuncShadow {
			return s, int(int8(op.args[1])), int(int8(op.args[2])), true
		}
	}
	return 0, 0, 0, false
}

func (d *gameData) initAnimation(iscriptID int) (int, error) {
	b := d.iscript
	off := int(binary.LittleEndian.Uint32(b))
	for {
		id := int(int16(binary.LittleEndian.Uint16(b[off:])))
		if id == -1 {
			return 0, fmt.Errorf("iscript %d not found", iscriptID)
		}
		header := int(binary.LittleEndian.Uint16(b[off+2:]))
		off += 4
		if id != iscriptID {
			continue
		}
		addr := int(binary.LittleEndian.Uint16(b[header+8:]))
		if addr == 0 {
			return 0, errors.New("iscript has no init animation")
		}
		return addr, nil
	}
}

type iscriptOp struct {
	name string
	args []int
}

var iscriptOps = []struct{ name, operands string }{
	{"playfram", "2"}, {"playframtile", "2"}, {"sethorpos", "1"}, {"setvertpos", "1"}, {"setpos", "11"},
	{"wait", "1"}, {"waitrand", "11"}, {"goto", "j"}, {"imgol", "211"}, {"imgul", "211"}, {"imgolorig", "2"},
	{"switchul", "2"}, {"__0c", ""}, {"imgoluselo", "211"}, {"imguluselo", "211"}, {"sprol", "211"},
	{"highsprol", "211"}, {"lowsprul", "211"}, {"uflunstable", "2"}, {"spruluselo", "211"}, {"sprul", "211"},
	{"sproluselo", "21"}, {"end", "e"}, {"setflipstate", "1"}, {"playsnd", "2"}, {"playsndrand", "v"},
	{"playsndbtwn", "22"}, {"domissiledmg", ""}, {"attackmelee", "v"}, {"followmaingraphic", ""},
	{"randcondjmp", "12"}, {"turnccwise", "1"}, {"turncwise", "1"}, {"turn1cwise", ""}, {"turnrand", "1"},
	{"setspawnframe", "1"}, {"sigorder", "1"}, {"attackwith", "1"}, {"attack", ""}, {"castspell", ""},
	{"useweapon", "1"}, {"move", "1"}, {"gotorepeatattk", ""}, {"engframe", "1"}, {"engset", "1"}, {"__2d", ""},
	{"nobrkcodestart", ""}, {"nobrkcodeend", ""}, {"ignorerest", ""}, {"attkshiftproj", "1"},
	{"tmprmgraphicstart", ""}, {"tmprmgraphicend", ""}, {"setfldirect", "1"}, {"call", "2"}, {"return", ""},
	{"setflspeed", "2"}, {"creategasoverlays", "1"}, {"pwrupcondjmp", "2"}, {"trgtrangecondjmp", "22"},
	{"trgtarccondjmp", "222"}, {"curdirectcondjmp", "222"}, {"imgulnextid", "11"}, {"__3e", ""},
	{"liftoffcondjmp", "2"}, {"warpoverlay", "2"}, {"orderdone", "1"}, {"grdsprol", "211"}, {"__43", ""},
	{"dogrddamage", ""},
}

// disassemble decodes a straight-line run of iscript until goto, end or max ops. Operands are raw
// unsigned values; callers sign-extend the ones that are signed.
func (d *gameData) disassemble(addr, max int) []iscriptOp {
	b := d.iscript
	var out []iscriptOp
	for len(out) < max && addr < len(b) {
		code := int(b[addr])
		if code >= len(iscriptOps) {
			break
		}
		spec := iscriptOps[code]
		op := iscriptOp{name: spec.name}
		p := addr + 1
		stop := false
		for _, kind := range spec.operands {
			switch kind {
			case '1':
				op.args = append(op.args, int(b[p]))
				p++
			case '2', 'j':
				op.args = append(op.args, int(binary.LittleEndian.Uint16(b[p:])))
				p += 2
				stop = stop || kind == 'j'
			case 'v':
				n := int(b[p])
				p += 1 + 2*n
			case 'e':
				stop = true
			}
		}
		out = append(out, op)
		if stop {
			break
		}
		addr = p
	}
	return out
}
