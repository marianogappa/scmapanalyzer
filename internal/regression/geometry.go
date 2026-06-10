package regression

import (
	"math"

	"github.com/marianogappa/scmapanalyzer/replaymap"
)

// bbox returns the inclusive integer bounding box of a polygon.
func bbox(poly []replaymap.TilePoint) (minX, minY, maxX, maxY int) {
	minX, minY = 1<<30, 1<<30
	maxX, maxY = -(1 << 30), -(1 << 30)
	for _, p := range poly {
		if p.X < minX {
			minX = p.X
		}
		if p.Y < minY {
			minY = p.Y
		}
		if p.X > maxX {
			maxX = p.X
		}
		if p.Y > maxY {
			maxY = p.Y
		}
	}
	return
}

// pointInPolygon uses the even-odd ray-casting rule. Coordinates are treated as
// continuous; we sample at cell centers (x+0.5, y+0.5) when rasterizing.
func pointInPolygon(px, py float64, poly []replaymap.TilePoint) bool {
	in := false
	n := len(poly)
	for i, j := 0, n-1; i < n; j, i = i, i+1 {
		xi, yi := float64(poly[i].X), float64(poly[i].Y)
		xj, yj := float64(poly[j].X), float64(poly[j].Y)
		if (yi > py) != (yj > py) {
			xCross := (xj-xi)*(py-yi)/(yj-yi) + xi
			if px < xCross {
				in = !in
			}
		}
	}
	return in
}

// polygonIoU rasterizes both polygons over their combined bounding box at
// minitile resolution and returns intersection-over-union of the filled areas.
// Returns 1.0 if both are empty, 0.0 if exactly one is empty.
func polygonIoU(a, b []replaymap.TilePoint) float64 {
	if len(a) < 3 && len(b) < 3 {
		return 1
	}
	if len(a) < 3 || len(b) < 3 {
		return 0
	}
	aMinX, aMinY, aMaxX, aMaxY := bbox(a)
	bMinX, bMinY, bMaxX, bMaxY := bbox(b)
	minX, minY := min(aMinX, bMinX), min(aMinY, bMinY)
	maxX, maxY := max(aMaxX, bMaxX), max(aMaxY, bMaxY)

	var inter, union int
	for y := minY; y <= maxY; y++ {
		py := float64(y) + 0.5
		for x := minX; x <= maxX; x++ {
			px := float64(x) + 0.5
			inA := pointInPolygon(px, py, a)
			inB := pointInPolygon(px, py, b)
			if inA && inB {
				inter++
			}
			if inA || inB {
				union++
			}
		}
	}
	if union == 0 {
		return 1
	}
	return float64(inter) / float64(union)
}

// centerRadiusFrac defines the "map center" zone as a circle whose radius is
// this fraction of the shorter map dimension. Mirrors expansion_names.go's
// notion of a center circle (diameter ~20% of the map).
const centerRadiusFrac = 0.11

// centerBaseCount counts bases (starts + expas) whose center lies inside the
// map's center circle. This is the metric for the 1-4 bug: two distinct bases
// in the center should collapse to one.
func centerBaseCount(out *replaymap.AnalyzeOutput) int {
	w, h := out.Debug.WidthMinitiles, out.Debug.HeightMinitiles
	cx, cy := float64(w)/2, float64(h)/2
	radius := centerRadiusFrac * math.Min(float64(w), float64(h))
	n := 0
	for _, b := range allBasesOut(out.Result) {
		dx, dy := float64(b.CenterTile.X)-cx, float64(b.CenterTile.Y)-cy
		if math.Sqrt(dx*dx+dy*dy) <= radius {
			n++
		}
	}
	return n
}

func allBasesOut(r *replaymap.Result) []replaymap.BasePolygon {
	out := make([]replaymap.BasePolygon, 0, len(r.Starts)+len(r.Expas))
	out = append(out, r.Starts...)
	out = append(out, r.Expas...)
	return out
}

func centerDist(a, b replaymap.TilePoint) float64 {
	dx := float64(a.X - b.X)
	dy := float64(a.Y - b.Y)
	return math.Sqrt(dx*dx + dy*dy)
}
