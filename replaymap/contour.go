package replaymap

import "math"

// contourSimplifyEpsilon is the Douglas-Peucker tolerance (minitiles) applied to
// traced outlines: removes staircase noise while keeping cliff-hugging shape.
const contourSimplifyEpsilon = 0.9

// maskOutline returns the outer boundary of the largest 4-connected component
// of mask as a simplified polygon whose vertices sit on minitile corners.
func maskOutline(w, h int, mask []bool) []TilePoint {
	comp := largestComponent(w, h, mask)
	if comp == nil {
		return nil
	}
	in := func(x, y int) bool { return x >= 0 && y >= 0 && x < w && y < h && comp[y*w+x] }

	start := -1
	for i, v := range comp {
		if v {
			start = i
			break
		}
	}
	sx, sy := start%w, start/w

	// Walk cell edges clockwise (y down) with the region on the right. At a
	// pinch corner prefer the right turn so diagonal-only contacts split.
	type pt struct{ x, y int }
	dirs := [4]pt{{1, 0}, {0, 1}, {-1, 0}, {0, -1}}
	edgeOK := func(p pt, d int) bool {
		switch d {
		case 0:
			return in(p.x, p.y) && !in(p.x, p.y-1)
		case 1:
			return in(p.x-1, p.y) && !in(p.x, p.y)
		case 2:
			return in(p.x-1, p.y-1) && !in(p.x-1, p.y)
		default:
			return in(p.x, p.y-1) && !in(p.x-1, p.y-1)
		}
	}
	cur := pt{sx, sy}
	d := 0
	var path []pt
	for guard := 0; guard < 4*w*h; guard++ {
		path = append(path, cur)
		cur = pt{cur.x + dirs[d].x, cur.y + dirs[d].y}
		if cur == (pt{sx, sy}) && d == 3 {
			break
		}
		for _, turn := range [3]int{1, 0, 3} {
			nd := (d + turn) % 4
			if edgeOK(cur, nd) {
				d = nd
				break
			}
		}
	}

	var corners []TilePoint
	for i := range path {
		p := path[(i+len(path)-1)%len(path)]
		c := path[i]
		nx := path[(i+1)%len(path)]
		if (c.x-p.x)*(nx.y-c.y)-(c.y-p.y)*(nx.x-c.x) != 0 {
			corners = append(corners, TilePoint{X: c.x, Y: c.y})
		}
	}
	return simplifyClosed(corners, contourSimplifyEpsilon)
}

func largestComponent(w, h int, mask []bool) []bool {
	label := make([]int32, w*h)
	bestID, bestSize := int32(0), 0
	next := int32(1)
	var queue []int
	for i, v := range mask {
		if !v || label[i] != 0 {
			continue
		}
		id := next
		next++
		label[i] = id
		queue = append(queue[:0], i)
		for q := 0; q < len(queue); q++ {
			x, y := queue[q]%w, queue[q]/w
			for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				nx, ny := x+d[0], y+d[1]
				if nx < 0 || ny < 0 || nx >= w || ny >= h {
					continue
				}
				ni := ny*w + nx
				if mask[ni] && label[ni] == 0 {
					label[ni] = id
					queue = append(queue, ni)
				}
			}
		}
		if len(queue) > bestSize {
			bestSize, bestID = len(queue), id
		}
	}
	if bestSize == 0 {
		return nil
	}
	out := make([]bool, w*h)
	for i, l := range label {
		out[i] = l == bestID
	}
	return out
}

func simplifyClosed(pts []TilePoint, eps float64) []TilePoint {
	if len(pts) < 4 {
		return pts
	}
	far := 0
	best := -1.0
	for i, p := range pts {
		d := math.Hypot(float64(p.X-pts[0].X), float64(p.Y-pts[0].Y))
		if d > best {
			best, far = d, i
		}
	}
	a := douglasPeucker(pts[:far+1], eps)
	b := douglasPeucker(append(append([]TilePoint{}, pts[far:]...), pts[0]), eps)
	return append(a[:len(a)-1], b[:len(b)-1]...)
}

func douglasPeucker(pts []TilePoint, eps float64) []TilePoint {
	if len(pts) < 3 {
		return pts
	}
	a, b := pts[0], pts[len(pts)-1]
	idx, best := 0, -1.0
	for i := 1; i < len(pts)-1; i++ {
		if d := segmentDistance(pts[i], a, b); d > best {
			best, idx = d, i
		}
	}
	if best <= eps {
		return []TilePoint{a, b}
	}
	left := douglasPeucker(pts[:idx+1], eps)
	right := douglasPeucker(pts[idx:], eps)
	return append(left[:len(left)-1], right...)
}

func segmentDistance(p, a, b TilePoint) float64 {
	px, py := float64(p.X), float64(p.Y)
	ax, ay := float64(a.X), float64(a.Y)
	bx, by := float64(b.X), float64(b.Y)
	dx, dy := bx-ax, by-ay
	l2 := dx*dx + dy*dy
	if l2 == 0 {
		return math.Hypot(px-ax, py-ay)
	}
	t := math.Max(0, math.Min(1, ((px-ax)*dx+(py-ay)*dy)/l2))
	return math.Hypot(px-ax-t*dx, py-ay-t*dy)
}
