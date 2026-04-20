package grp

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
)

type FrameInfo struct {
	XOffset int
	YOffset int
	Width   int
	Height  int
	Offset  int
}

type File struct {
	FrameCount int
	CanvasW    int
	CanvasH    int
	Frames     []FrameInfo
	Data       []byte
}

func Parse(data []byte) (*File, error) {
	if len(data) < 6 {
		return nil, fmt.Errorf("invalid grp size: %d", len(data))
	}
	fc := int(binary.LittleEndian.Uint16(data[0:2]))
	w := int(binary.LittleEndian.Uint16(data[2:4]))
	h := int(binary.LittleEndian.Uint16(data[4:6]))
	if fc <= 0 {
		return nil, fmt.Errorf("invalid frame count: %d", fc)
	}
	if len(data) < 6+fc*8 {
		return nil, fmt.Errorf("grp truncated frame table")
	}

	frames := make([]FrameInfo, fc)
	for i := 0; i < fc; i++ {
		base := 6 + i*8
		frames[i] = FrameInfo{
			XOffset: int(data[base+0]),
			YOffset: int(data[base+1]),
			Width:   int(data[base+2]),
			Height:  int(data[base+3]),
			Offset:  int(binary.LittleEndian.Uint32(data[base+4 : base+8])),
		}
	}
	return &File{
		FrameCount: fc,
		CanvasW:    w,
		CanvasH:    h,
		Frames:     frames,
		Data:       data,
	}, nil
}

func FirstFrameOnly(data []byte) ([]byte, error) {
	f, err := Parse(data)
	if err != nil {
		return nil, err
	}
	first := f.Frames[0]
	if first.Offset <= 0 || first.Offset >= len(data) {
		return nil, fmt.Errorf("first frame offset out of range: %d", first.Offset)
	}
	end := len(data)
	for i := 1; i < len(f.Frames); i++ {
		off := f.Frames[i].Offset
		if off > first.Offset && off < end {
			end = off
		}
	}
	chunk := data[first.Offset:end]

	out := make([]byte, 0, 14+len(chunk))
	out = append(out, 0, 0, 0, 0, 0, 0)
	binary.LittleEndian.PutUint16(out[0:2], 1)
	binary.LittleEndian.PutUint16(out[2:4], uint16(f.CanvasW))
	binary.LittleEndian.PutUint16(out[4:6], uint16(f.CanvasH))

	table := make([]byte, 8)
	table[0] = byte(first.XOffset)
	table[1] = byte(first.YOffset)
	table[2] = byte(first.Width)
	table[3] = byte(first.Height)
	binary.LittleEndian.PutUint32(table[4:8], 14)
	out = append(out, table...)
	out = append(out, chunk...)
	return out, nil
}

func DecodeFrameRGBA(data []byte, frameIdx int, palette color.Palette) (*image.RGBA, error) {
	f, err := Parse(data)
	if err != nil {
		return nil, err
	}
	if frameIdx < 0 || frameIdx >= f.FrameCount {
		return nil, fmt.Errorf("frame index out of range: %d", frameIdx)
	}
	frame := f.Frames[frameIdx]
	if frame.Offset <= 0 || frame.Offset >= len(f.Data) {
		return nil, fmt.Errorf("frame offset out of range: %d", frame.Offset)
	}
	if frame.Height <= 0 || frame.Width <= 0 {
		return image.NewRGBA(image.Rect(0, 0, f.CanvasW, f.CanvasH)), nil
	}
	lineTableEnd := frame.Offset + frame.Height*2
	if lineTableEnd > len(f.Data) {
		return nil, fmt.Errorf("frame line table truncated")
	}

	img := image.NewRGBA(image.Rect(0, 0, f.CanvasW, f.CanvasH))
	for y := 0; y < frame.Height; y++ {
		rowRel := int(binary.LittleEndian.Uint16(f.Data[frame.Offset+y*2 : frame.Offset+y*2+2]))
		ptr := frame.Offset + rowRel
		x := 0
		for x < frame.Width && ptr < len(f.Data) {
			op := f.Data[ptr]
			ptr++
			switch {
			case op >= 0x80:
				x += int(op - 0x80)
			case op >= 0x40:
				run := int(op - 0x40)
				if ptr >= len(f.Data) {
					break
				}
				idx := f.Data[ptr]
				ptr++
				c := palette[idx]
				for i := 0; i < run && x < frame.Width; i++ {
					px := frame.XOffset + x
					py := frame.YOffset + y
					if image.Pt(px, py).In(img.Rect) {
						img.Set(px, py, c)
					}
					x++
				}
			case op > 0:
				run := int(op)
				for i := 0; i < run && x < frame.Width && ptr < len(f.Data); i++ {
					idx := f.Data[ptr]
					ptr++
					px := frame.XOffset + x
					py := frame.YOffset + y
					if image.Pt(px, py).In(img.Rect) {
						img.Set(px, py, palette[idx])
					}
					x++
				}
			default:
				x = frame.Width
			}
		}
	}
	return img, nil
}
