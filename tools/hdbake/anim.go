package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"strings"
)

// anim is one HD2/anim/main_NNN.anim. Frame rects, offsets and the grp box are stored at HD (4x)
// scale while the HD2 textures are half that, so every coordinate is halved on read: the result is
// at HD2 scale, 2 sprite pixels per map pixel.
type anim struct {
	frames []animFrame
	grpW   int
	grpH   int
	layers map[string]*image.NRGBA
}

type animFrame struct {
	rect image.Rectangle
	// origin is the sprite position inside rect, relative to rect.Min.
	origin image.Point
}

const (
	animHeaderSize = 332
	animLayerSlots = 10
)

func parseAnim(raw []byte) (*anim, error) {
	if len(raw) < animHeaderSize+12+animLayerSlots*12 || string(raw[:4]) != "ANIM" {
		return nil, errors.New("anim: bad header")
	}
	names := make([]string, animLayerSlots)
	for i := range names {
		names[i] = strings.TrimRight(string(raw[12+i*32:12+(i+1)*32]), "\x00")
	}
	e := raw[animHeaderSize:]
	n := int(binary.LittleEndian.Uint16(e))
	if n == 0 {
		return nil, errors.New("anim: entry is a reference, not frames")
	}
	grpW, grpH := int(binary.LittleEndian.Uint16(e[4:])), int(binary.LittleEndian.Uint16(e[6:]))
	a := &anim{grpW: grpW / 2, grpH: grpH / 2, layers: map[string]*image.NRGBA{}}
	framePtr := int(binary.LittleEndian.Uint32(e[8:]))
	for i := range n {
		f := raw[framePtr+i*16:]
		u := func(o int) int { return int(binary.LittleEndian.Uint16(f[o:])) }
		s := func(o int) int { return int(int16(binary.LittleEndian.Uint16(f[o:]))) }
		r := image.Rect(u(0)/2, u(2)/2, (u(0)+u(8))/2, (u(2)+u(10))/2)
		a.frames = append(a.frames, animFrame{rect: r, origin: image.Pt((grpW-2*s(4))/4, (grpH-2*s(6))/4)})
	}
	for i, name := range names {
		if name != "diffuse" && name != "teamcolor" {
			continue
		}
		o := 12 + i*12
		ptr, size := int(binary.LittleEndian.Uint32(e[o:])), int(binary.LittleEndian.Uint32(e[o+4:]))
		if ptr == 0 || size == 0 {
			continue
		}
		if ptr+size > len(raw) {
			return nil, fmt.Errorf("anim: %s layer out of bounds", name)
		}
		blob, ok := concatDDS(raw[ptr : ptr+size])
		if !ok {
			return nil, fmt.Errorf("anim: %s layer is not DDS", name)
		}
		img, err := decodeDDS(blob)
		if err != nil {
			return nil, fmt.Errorf("anim: %s: %w", name, err)
		}
		a.layers[name] = img
	}
	if a.layers["diffuse"] == nil {
		return nil, errors.New("anim: no diffuse layer")
	}
	if t := a.layers["teamcolor"]; t != nil && t.Bounds() != a.layers["diffuse"].Bounds() {
		delete(a.layers, "teamcolor")
	}
	return a, nil
}

func (a *anim) frame(i int) (animFrame, error) {
	if i < 0 || i >= len(a.frames) {
		return animFrame{}, fmt.Errorf("frame %d out of range (%d frames)", i, len(a.frames))
	}
	f := a.frames[i]
	if !f.rect.In(a.layers["diffuse"].Bounds()) {
		return animFrame{}, fmt.Errorf("frame %d rect outside texture", i)
	}
	return f, nil
}
