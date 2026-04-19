package tiletagger

import (
	"image"
	"image/color"
	"slices"
	"testing"
)

func TestClassifyTilesAccumulatesWalkableIDs(t *testing.T) {
	tiles := []uint16{
		11, 22,
	}
	overlay := image.NewRGBA(image.Rect(0, 0, 20, 10))
	fillRect(overlay, image.Rect(0, 0, 10, 10), color.RGBA{R: 255, G: 255, B: 0, A: 255})
	fillRect(overlay, image.Rect(10, 0, 20, 10), color.RGBA{R: 255, G: 255, B: 0, A: 255})

	wallIDs, rampIDs, walkableIDs := classifyTiles(
		tiles,
		2,
		1,
		overlay,
		0.45,
		0.55,
		1,
	)

	if len(wallIDs) != 0 {
		t.Fatalf("expected no wall IDs, got %v", wallIDs)
	}
	if len(rampIDs) != 0 {
		t.Fatalf("expected no ramp IDs, got %v", rampIDs)
	}
	want := []uint16{11, 22}
	if !slices.Equal(walkableIDs, want) {
		t.Fatalf("unexpected walkable IDs: got %v want %v", walkableIDs, want)
	}
}

func TestIsBrightYellow(t *testing.T) {
	tcs := []struct {
		name string
		in   color.RGBA
		want bool
	}{
		{
			name: "pure yellow is accepted",
			in:   color.RGBA{R: 255, G: 255, B: 0, A: 255},
			want: true,
		},
		{
			name: "strong yellow with slight blue noise is accepted",
			in:   color.RGBA{R: 210, G: 210, B: 100, A: 255},
			want: true,
		},
		{
			name: "purple is rejected",
			in:   color.RGBA{R: 180, G: 20, B: 220, A: 255},
			want: false,
		},
		{
			name: "dark color is rejected",
			in:   color.RGBA{R: 120, G: 120, B: 0, A: 255},
			want: false,
		},
	}

	for _, tc := range tcs {
		if got := isBrightYellow(tc.in); got != tc.want {
			t.Fatalf("%s: isBrightYellow(%v)=%v want %v", tc.name, tc.in, got, tc.want)
		}
	}
}

func fillRect(img *image.RGBA, rect image.Rectangle, c color.RGBA) {
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}
