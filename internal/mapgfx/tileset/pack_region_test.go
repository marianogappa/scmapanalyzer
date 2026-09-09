package tileset

import (
	"image"
	"image/color"
	"testing"
)

func testPack() *Pack {
	pal := make(color.Palette, 256)
	for i := range pal {
		pal[i] = color.RGBA{R: uint8(i), G: uint8(i), B: uint8(i), A: 255}
	}
	minitiles := make([]byte, 2*64)
	for i := 0; i < 64; i++ {
		minitiles[64+i] = 7
	}
	megatiles := make([]uint16, 2*16)
	for i := 0; i < 16; i++ {
		megatiles[16+i] = 1 << 1
	}
	return &Pack{
		Palette:      pal,
		TileMegatile: []uint16{0, 1},
		Megatiles:    megatiles,
		Minitiles:    minitiles,
	}
}

func TestRenderPackRegionToPaletted(t *testing.T) {
	p := testPack()
	tiles := []uint16{
		0, 1,
		1, 0,
	}
	img, err := RenderPackRegionToPaletted(p, 2, 2, tiles, 1, 0, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if want := image.Rect(32, 0, 64, 64); img.Bounds() != want {
		t.Fatalf("bounds = %v, want %v", img.Bounds(), want)
	}
	if got := img.ColorIndexAt(40, 8); got != 7 {
		t.Errorf("pixel in tile (1,0) = %d, want 7", got)
	}
	if got := img.ColorIndexAt(40, 40); got != 0 {
		t.Errorf("pixel in tile (1,1) = %d, want 0", got)
	}

	full, err := RenderPackToPaletted(p, 2, 2, tiles)
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < 64; y++ {
		for x := 32; x < 64; x++ {
			if full.ColorIndexAt(x, y) != img.ColorIndexAt(x, y) {
				t.Fatalf("region differs from full render at (%d,%d)", x, y)
			}
		}
	}
}

func TestRenderPackRegionToPalettedBounds(t *testing.T) {
	p := testPack()
	tiles := []uint16{0, 1, 1, 0}
	for _, r := range [][4]int{
		{-1, 0, 1, 1},
		{0, -1, 1, 1},
		{0, 0, 0, 1},
		{0, 0, 1, 0},
		{2, 0, 1, 1},
		{0, 0, 3, 1},
	} {
		if _, err := RenderPackRegionToPaletted(p, 2, 2, tiles, r[0], r[1], r[2], r[3]); err == nil {
			t.Errorf("rect %v: expected error", r)
		}
	}
}
