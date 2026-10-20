// Minimal DDS decoding for DXT1 / BC1 and DXT5 / BC3 pixel data, the formats HD2 anims use.
package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
)

const ddsMagic = 0x20534444 // 'DDS '

// decodeDXT5 parses a DDS file and returns an NRGBA image (premultiplied-ish RGB, straight A).
func decodeDXT5(dds []byte) (*image.NRGBA, error) {
	if len(dds) < 128 {
		return nil, errors.New("dds: too small")
	}
	if binary.LittleEndian.Uint32(dds[0:4]) != ddsMagic {
		return nil, errors.New("dds: bad magic")
	}
	height := int(binary.LittleEndian.Uint32(dds[12:16]))
	width := int(binary.LittleEndian.Uint32(dds[16:20]))
	// DDSPixelFormat starts at offset 76 in standard 124-byte header (magic + 124-byte header = 128 total).
	fourCC := string(dds[84:88])
	if fourCC != "DXT5" && fourCC != "BC3\x00" {
		return nil, fmt.Errorf("dds: unsupported fourCC %q", fourCC)
	}
	dataOff := 128
	if len(dds) < dataOff+((width+3)/4)*((height+3)/4)*16 {
		return nil, errors.New("dds: truncated pixel data")
	}
	dst := image.NewNRGBA(image.Rect(0, 0, width, height))
	src := dds[dataOff:]
	blockW := (width + 3) / 4
	blockH := (height + 3) / 4
	for by := 0; by < blockH; by++ {
		for bx := 0; bx < blockW; bx++ {
			block := src[(by*blockW+bx)*16 : (by*blockW+bx)*16+16]
			decodeDXT5Block(block, dst, bx*4, by*4, width, height)
		}
	}
	return dst, nil
}

func dxt5AlphaTable(a0, a1 uint8) [8]uint8 {
	var t [8]uint8
	t[0] = a0
	t[1] = a1
	if a0 > a1 {
		for i := 1; i < 7; i++ {
			t[i+1] = uint8((uint16(a0)*(7-uint16(i)) + uint16(a1)*uint16(i)) / 7)
		}
	} else {
		for i := 1; i < 5; i++ {
			t[i+1] = uint8((uint16(a0)*(5-uint16(i)) + uint16(a1)*uint16(i)) / 5)
		}
		t[6] = 0
		t[7] = 255
	}
	return t
}

func decodeDXT5Block(block []byte, dst *image.NRGBA, ox, oy, w, h int) {
	if len(block) < 16 {
		return
	}
	alpha := dxt5AlphaTable(block[0], block[1])
	alphaBits := uint64(block[2]) | uint64(block[3])<<8 | uint64(block[4])<<16 |
		uint64(block[5])<<24 | uint64(block[6])<<32 | uint64(block[7])<<40

	cr, cg, cb := dxtColorTable(block[8:12])
	bits := binary.LittleEndian.Uint32(block[12:16])

	for py := 0; py < 4; py++ {
		for px := 0; px < 4; px++ {
			x, y := ox+px, oy+py
			if x >= w || y >= h {
				continue
			}
			idx := py*4 + px
			colorIdx := (bits >> (2 * idx)) & 3
			aIdx := (alphaBits >> (3 * idx)) & 7
			a := uint32(alpha[aIdx])
			r := cr[colorIdx]
			g := cg[colorIdx]
			b := cb[colorIdx]
			off := dst.PixOffset(x, y)
			dst.Pix[off+0] = uint8(r)
			dst.Pix[off+1] = uint8(g)
			dst.Pix[off+2] = uint8(b)
			dst.Pix[off+3] = uint8(a)
		}
	}
}

func dxtColorTable(b []byte) (cr, cg, cb [4]uint32) {
	c0 := binary.LittleEndian.Uint16(b[0:2])
	c1 := binary.LittleEndian.Uint16(b[2:4])
	r0, g0, b0 := rgb565(c0)
	r1, g1, b1 := rgb565(c1)
	cr[0], cg[0], cb[0] = uint32(r0), uint32(g0), uint32(b0)
	cr[1], cg[1], cb[1] = uint32(r1), uint32(g1), uint32(b1)
	if c0 > c1 {
		cr[2] = (2*uint32(r0) + uint32(r1)) / 3
		cg[2] = (2*uint32(g0) + uint32(g1)) / 3
		cb[2] = (2*uint32(b0) + uint32(b1)) / 3
		cr[3] = (uint32(r0) + 2*uint32(r1)) / 3
		cg[3] = (uint32(g0) + 2*uint32(g1)) / 3
		cb[3] = (uint32(b0) + 2*uint32(b1)) / 3
	} else {
		cr[2] = (uint32(r0) + uint32(r1)) / 2
		cg[2] = (uint32(g0) + uint32(g1)) / 2
		cb[2] = (uint32(b0) + uint32(b1)) / 2
	}
	return cr, cg, cb
}

// decodeDXT1 parses a DDS file with BC1 data. Index 3 of a block with c0 <= c1 is
// transparent black, per the format.
func decodeDXT1(dds []byte) (*image.NRGBA, error) {
	if len(dds) < 128 {
		return nil, errors.New("dds: too small")
	}
	if binary.LittleEndian.Uint32(dds[0:4]) != ddsMagic {
		return nil, errors.New("dds: bad magic")
	}
	height := int(binary.LittleEndian.Uint32(dds[12:16]))
	width := int(binary.LittleEndian.Uint32(dds[16:20]))
	blockW := (width + 3) / 4
	blockH := (height + 3) / 4
	src := dds[128:]
	if len(src) < blockW*blockH*8 {
		return nil, errors.New("dds: truncated pixel data")
	}
	dst := image.NewNRGBA(image.Rect(0, 0, width, height))
	for by := 0; by < blockH; by++ {
		for bx := 0; bx < blockW; bx++ {
			block := src[(by*blockW+bx)*8:]
			cr, cg, cb := dxtColorTable(block[0:4])
			transparent3 := binary.LittleEndian.Uint16(block[0:2]) <= binary.LittleEndian.Uint16(block[2:4])
			bits := binary.LittleEndian.Uint32(block[4:8])
			for py := 0; py < 4; py++ {
				for px := 0; px < 4; px++ {
					x, y := bx*4+px, by*4+py
					if x >= width || y >= height {
						continue
					}
					i := (bits >> (2 * (py*4 + px))) & 3
					a := uint8(255)
					if transparent3 && i == 3 {
						a = 0
					}
					off := dst.PixOffset(x, y)
					dst.Pix[off+0] = uint8(cr[i])
					dst.Pix[off+1] = uint8(cg[i])
					dst.Pix[off+2] = uint8(cb[i])
					dst.Pix[off+3] = a
				}
			}
		}
	}
	return dst, nil
}

func rgb565(v uint16) (r, g, b uint8) {
	r = uint8((v >> 11) * 255 / 31)
	g = uint8(((v >> 5) & 0x3F) * 255 / 63)
	b = uint8((v & 0x1F) * 255 / 31)
	return
}

// isDDS returns true if b starts with a DDS magic header.
func isDDS(b []byte) bool {
	return len(b) >= 4 && binary.LittleEndian.Uint32(b[0:4]) == ddsMagic
}

// fourCC returns the DDS pixel format FourCC or empty if not present.
func fourCC(dds []byte) string {
	if len(dds) < 88 {
		return ""
	}
	return string(dds[84:88])
}

// decodeDDS decodes DDS if DXT1 or DXT5; otherwise returns error.
func decodeDDS(dds []byte) (*image.NRGBA, error) {
	if !isDDS(dds) {
		return nil, errors.New("dds: not a DDS blob")
	}
	switch fourCC(dds) {
	case "DXT5", "BC3\x00":
		return decodeDXT5(dds)
	case "DXT1", "BC1\x00":
		return decodeDXT1(dds)
	default:
		return nil, fmt.Errorf("dds: unsupported fourCC %q", fourCC(dds))
	}
}

// concatDDS finds the first DDS magic in buf and returns that slice.
func concatDDS(buf []byte) ([]byte, bool) {
	i := bytes.Index(buf, []byte{'D', 'D', 'S', ' '})
	if i < 0 {
		return nil, false
	}
	return buf[i:], true
}
