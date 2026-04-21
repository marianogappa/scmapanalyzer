package replaymap

import (
	"math"
	"testing"
)

func TestRayAngleMatchesOclockSectors(t *testing.T) {
	const wt, ht = 128, 128
	cx := wt * 16
	cy := ht * 16
	// East of center → oclock 3
	if g := oclock(wt, ht, cx+500, cy); g != 3 {
		t.Fatalf("east: got %d want 3", g)
	}
	// South-east sector center ~45° → 5
	px := cx + int(400*math.Cos(45*math.Pi/180))
	py := cy + int(400*math.Sin(45*math.Pi/180))
	if g := oclock(wt, ht, px, py); g != 5 {
		t.Fatalf("45deg: got %d want 5", g)
	}
}

func TestOclockFloatRange(t *testing.T) {
	const wt, ht = 128, 128
	cx := wt * 16
	cy := ht * 16
	for _, tc := range []struct {
		dx, dy int
	}{
		{500, 0},
		{0, 500},
		{-500, 0},
		{350, 350},
	} {
		v := oclockFloat(wt, ht, cx+tc.dx, cy+tc.dy)
		if v <= 0 || v > 12 {
			t.Fatalf("oclockFloat(%d,%d)=%f out of (0,12]", tc.dx, tc.dy, v)
		}
	}
}

func TestNearestUniformDialHour(t *testing.T) {
	if g := nearestUniformDialHour(3.75); g != 4 {
		t.Fatalf("3.75 → got %d want 4", g)
	}
	if g := nearestUniformDialHour(3.0); g != 3 {
		t.Fatalf("3.0 → got %d want 3", g)
	}
}
