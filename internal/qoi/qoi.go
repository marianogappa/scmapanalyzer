// Package qoi implements QOI (Quite OK Image) encode/decode for RGBA8.
// Based on https://github.com/phoboslab/qoi (MIT).
package qoi

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
)

const (
	magic        = "qoif"
	headerSize   = 14
	paddingSize  = 8
	opIndex      = 0x00
	opDiff       = 0x40
	opLuma       = 0x80
	opRun        = 0xc0
	opRGB        = 0xfe
	opRGBA       = 0xff
	mask2        = 0xc0
)

var padding = [paddingSize]byte{0, 0, 0, 0, 0, 0, 0, 1}

type rgba struct{ r, g, b, a uint8 }

func (c rgba) hash() int {
	return int(c.r*3+c.g*5+c.b*7+c.a*11) & 63
}

// EncodeRGBA encodes width×height RGBA8 pixels (row-major, 4 bytes per pixel) as QOI.
func EncodeRGBA(width, height int, pixels []byte) ([]byte, error) {
	if width <= 0 || height <= 0 {
		return nil, errors.New("qoi: invalid dimensions")
	}
	if len(pixels) != width*height*4 {
		return nil, errors.New("qoi: pixel length mismatch")
	}

	maxSize := width*height*5 + headerSize + paddingSize
	b := make([]byte, 0, maxSize)
	buf := bytes.NewBuffer(b)
	_, _ = buf.WriteString(magic)
	_ = binary.Write(buf, binary.BigEndian, uint32(width))
	_ = binary.Write(buf, binary.BigEndian, uint32(height))
	_ = buf.WriteByte(4)
	_ = buf.WriteByte(0)

	var index [64]rgba
	var px, pxPrev rgba
	pxPrev = rgba{0, 0, 0, 255}
	run := 0
	pxEnd := len(pixels) - 4

	for pxPos := 0; pxPos < len(pixels); pxPos += 4 {
		px.r = pixels[pxPos]
		px.g = pixels[pxPos+1]
		px.b = pixels[pxPos+2]
		px.a = pixels[pxPos+3]

		if px == pxPrev {
			run++
			if run == 62 || pxPos == pxEnd {
				_ = buf.WriteByte(opRun | byte(run-1))
				run = 0
			}
			continue
		}
		if run > 0 {
			_ = buf.WriteByte(opRun | byte(run-1))
			run = 0
		}

		idxPos := px.hash()
		if index[idxPos] == px {
			_ = buf.WriteByte(opIndex | byte(idxPos))
			pxPrev = px
			continue
		}
		index[idxPos] = px

		if px.a == pxPrev.a {
			vr := int16(px.r) - int16(pxPrev.r)
			vg := int16(px.g) - int16(pxPrev.g)
			vb := int16(px.b) - int16(pxPrev.b)
			vgR := vr - vg
			vgB := vb - vg
			if vr > -3 && vr < 2 && vg > -3 && vg < 2 && vb > -3 && vb < 2 {
				_ = buf.WriteByte(opDiff | byte(vr+2)<<4 | byte(vg+2)<<2 | byte(vb+2))
			} else if vgR > -9 && vgR < 8 && vg > -33 && vg < 32 && vgB > -9 && vgB < 8 {
				_ = buf.WriteByte(opLuma | byte(vg+32))
				_ = buf.WriteByte(byte(vgR+8)<<4 | byte(vgB+8))
			} else {
				_ = buf.WriteByte(opRGB)
				_ = buf.WriteByte(px.r)
				_ = buf.WriteByte(px.g)
				_ = buf.WriteByte(px.b)
			}
		} else {
			_ = buf.WriteByte(opRGBA)
			_ = buf.WriteByte(px.r)
			_ = buf.WriteByte(px.g)
			_ = buf.WriteByte(px.b)
			_ = buf.WriteByte(px.a)
		}
		pxPrev = px
	}

	_, _ = buf.Write(padding[:])
	return buf.Bytes(), nil
}

// DecodeToNRGBA decodes QOI bytes into NRGBA (always 4 channels out).
func DecodeToNRGBA(data []byte) (*image.NRGBA, error) {
	if len(data) < headerSize+paddingSize {
		return nil, errors.New("qoi: too small")
	}
	if string(data[0:4]) != magic {
		return nil, errors.New("qoi: bad magic")
	}
	p := 4
	width := int(binary.BigEndian.Uint32(data[p : p+4]))
	p += 4
	height := int(binary.BigEndian.Uint32(data[p : p+4]))
	p += 4
	ch := data[p]
	p++
	_ = data[p] // colorspace
	p++
	if ch != 3 && ch != 4 {
		return nil, errors.New("qoi: unsupported channels")
	}
	if width <= 0 || height <= 0 {
		return nil, errors.New("qoi: invalid dimensions")
	}

	pixels := make([]byte, width*height*4)
	var index [64]rgba
	px := rgba{0, 0, 0, 255}
	run := 0
	chunksLen := len(data) - paddingSize
	out := 0

	for out < len(pixels) {
		if run > 0 {
			run--
		} else if p < chunksLen {
			b1 := data[p]
			p++
			switch {
			case b1 == opRGB:
				px.r = data[p]
				p++
				px.g = data[p]
				p++
				px.b = data[p]
				p++
			case b1 == opRGBA:
				px.r = data[p]
				p++
				px.g = data[p]
				p++
				px.b = data[p]
				p++
				px.a = data[p]
				p++
			case (b1 & mask2) == opIndex:
				px = index[b1&0x3f]
			case (b1 & mask2) == opDiff:
				px.r += uint8((b1 >> 4 & 3) - 2)
				px.g += uint8((b1 >> 2 & 3) - 2)
				px.b += uint8((b1 & 3) - 2)
			case (b1 & mask2) == opLuma:
				b2 := data[p]
				p++
				vg := int(b1&0x3f) - 32
				px.r += uint8(vg - 8 + int(b2>>4&0x0f))
				px.g += uint8(vg)
				px.b += uint8(vg - 8 + int(b2&0x0f))
			case (b1 & mask2) == opRun:
				run = int(b1 & 0x3f)
			default:
				return nil, errors.New("qoi: invalid op")
			}
			index[px.hash()] = px
		} else {
			return nil, errors.New("qoi: truncated stream")
		}
		o := out
		pixels[o] = px.r
		pixels[o+1] = px.g
		pixels[o+2] = px.b
		pixels[o+3] = px.a
		out += 4
	}

	return &image.NRGBA{
		Pix:    pixels,
		Stride: 4 * width,
		Rect:   image.Rect(0, 0, width, height),
	}, nil
}

// NRGBAFromImage converts any image to NRGBA (for encoding pipeline).
func NRGBAFromImage(src image.Image) *image.NRGBA {
	if n, ok := src.(*image.NRGBA); ok {
		return n
	}
	b := src.Bounds()
	dst := image.NewNRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dst.SetNRGBA(x, y, color.NRGBAModel.Convert(src.At(x, y)).(color.NRGBA))
		}
	}
	return dst
}
