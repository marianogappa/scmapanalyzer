package mapgfx

import (
	"image"
	"image/color"
	"testing"
)

func TestDownsampleFactor(t *testing.T) {
	cases := []struct {
		w, h, maxDim, want int
	}{
		{4096, 4096, 2560, 2},
		{4096, 3072, 2560, 2},
		{8192, 8192, 2560, 4},
		{2048, 2048, 2560, 1},
		{4096, 4096, 4096, 1},
		{4096, 4096, 1000, 8},
	}
	for _, c := range cases {
		if got := downsampleFactor(c.w, c.h, c.maxDim); got != c.want {
			t.Errorf("downsampleFactor(%d, %d, %d) = %d, want %d", c.w, c.h, c.maxDim, got, c.want)
		}
	}
}

func TestBoxDownsampleAverages(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			v := uint8(0)
			if (x+y)%2 == 0 {
				v = 200
			}
			src.SetRGBA(x, y, color.RGBA{R: v, G: v, B: v, A: 255})
		}
	}
	dst := boxDownsample(src, 2)
	if dst.Bounds() != image.Rect(0, 0, 2, 2) {
		t.Fatalf("bounds = %v, want 2x2", dst.Bounds())
	}
	got := dst.RGBAAt(0, 0)
	if got.R != 100 || got.G != 100 || got.B != 100 || got.A != 255 {
		t.Errorf("averaged pixel = %v, want {100 100 100 255}", got)
	}
}

func TestBoxDownsamplePaletted(t *testing.T) {
	pal := color.Palette{color.RGBA{A: 255}, color.RGBA{R: 40, G: 80, B: 120, A: 255}}
	src := image.NewPaletted(image.Rect(0, 0, 4, 4), pal)
	for i := range src.Pix {
		src.Pix[i] = 1
	}
	dst := boxDownsample(src, 2)
	got := dst.RGBAAt(1, 1)
	if got.R != 40 || got.G != 80 || got.B != 120 || got.A != 255 {
		t.Errorf("pixel = %v, want {40 80 120 255}", got)
	}
}
