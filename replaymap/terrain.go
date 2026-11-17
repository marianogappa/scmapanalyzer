package replaymap

import (
	"math"
	"sort"

	"github.com/marianogappa/scmapanalyzer/internal/mapgfx"
)

// Watershed merge thresholds, in minitiles. Two flooding areas that meet are
// kept apart (their meeting cells become a choke) only when the smaller one is
// a real room: big enough, wide enough, and the meeting point is a genuine
// constriction relative to its widest spot.
const (
	areaMergeMaxSize     = 80
	areaMergeMaxAltitude = 10.0
	areaMergeAltRatio    = 0.90
)

type terrainAreas struct {
	Width     int
	Height    int
	Passable  []bool
	Ramp      []bool
	Elevation []uint8
	Altitude  []float32
	Area      []int32
	AreaCount int
	Frontier  []frontierCell
}

type frontierCell struct {
	Idx  int
	A, B int32
}

func passableMask(w, h int, data []uint8) []bool {
	out := make([]bool, w*h)
	for i, f := range data {
		f &= 0x0F
		out[i] = f&mapgfx.FeatureWalkable != 0 || f&mapgfx.FeatureRamp != 0
	}
	return out
}

func rampMask(data []uint8) []bool {
	out := make([]bool, len(data))
	for i, f := range data {
		out[i] = f&mapgfx.FeatureRamp != 0
	}
	return out
}

// altitudeMap is a chamfer distance (in minitiles) from every passable cell to
// the nearest impassable cell or map border.
func altitudeMap(w, h int, passable []bool) []float32 {
	const diag = float32(math.Sqrt2)
	alt := make([]float32, w*h)
	for i, p := range passable {
		if p {
			alt[i] = math.MaxFloat32
		}
	}
	at := func(x, y int) float32 {
		if x < 0 || y < 0 || x >= w || y >= h {
			return 0
		}
		return alt[y*w+x]
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*w + x
			if alt[i] == 0 {
				continue
			}
			alt[i] = min(alt[i], at(x-1, y)+1, at(x, y-1)+1, at(x-1, y-1)+diag, at(x+1, y-1)+diag)
		}
	}
	for y := h - 1; y >= 0; y-- {
		for x := w - 1; x >= 0; x-- {
			i := y*w + x
			if alt[i] == 0 {
				continue
			}
			alt[i] = min(alt[i], at(x+1, y)+1, at(x, y+1)+1, at(x+1, y+1)+diag, at(x-1, y+1)+diag)
		}
	}
	return alt
}

// computeTerrainAreas partitions passable terrain into areas separated by
// chokes. Cells are flooded from the highest altitude (widest open ground)
// downwards; when two floods meet at a cell much narrower than the smaller
// flood's widest point, they stay separate and that cell is recorded as
// frontier. This is the watershed scheme popularized by BWEM.
func computeTerrainAreas(w, h int, data []uint8, elevation []uint8) *terrainAreas {
	passable := passableMask(w, h, data)
	alt := altitudeMap(w, h, passable)

	order := make([]int32, 0, w*h)
	for i, p := range passable {
		if p {
			order = append(order, int32(i))
		}
	}
	sort.Slice(order, func(a, b int) bool {
		if alt[order[a]] != alt[order[b]] {
			return alt[order[a]] > alt[order[b]]
		}
		return order[a] < order[b]
	})

	label := make([]int32, w*h)
	for i := range label {
		label[i] = -1
	}
	var parent []int32
	var size []int
	var maxAlt []float32
	var elev []uint8
	find := func(a int32) int32 {
		for parent[a] != a {
			parent[a] = parent[parent[a]]
			a = parent[a]
		}
		return a
	}

	var frontier []frontierCell
	var roots [4]int32
	for _, i32 := range order {
		i := int(i32)
		x, y := i%w, i/w
		n := 0
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			nx, ny := x+d[0], y+d[1]
			if nx < 0 || ny < 0 || nx >= w || ny >= h {
				continue
			}
			l := label[ny*w+nx]
			if l < 0 {
				continue
			}
			r := find(l)
			dup := false
			for k := 0; k < n; k++ {
				if roots[k] == r {
					dup = true
					break
				}
			}
			if !dup {
				roots[n] = r
				n++
			}
		}
		a := alt[i]
		var e uint8
		if elevation != nil {
			e = elevation[i]
		}
		rs := roots[:n]
		sort.Slice(rs, func(p, q int) bool {
			if (elev[rs[p]] == e) != (elev[rs[q]] == e) {
				return elev[rs[p]] == e
			}
			return size[rs[p]] > size[rs[q]]
		})
		if n == 0 || (elev[rs[0]] != e && size[rs[0]] >= areaMergeMaxSize) {
			id := int32(len(parent))
			parent = append(parent, id)
			size = append(size, 0)
			maxAlt = append(maxAlt, a)
			elev = append(elev, e)
			rs = append([]int32{id}, rs...)
		}
		cur := rs[0]
		for _, other := range rs[1:] {
			other = find(other)
			cur = find(cur)
			if other == cur {
				continue
			}
			small, big := other, cur
			if size[small] > size[big] {
				small, big = big, small
			}
			lowPeak := min(maxAlt[small], maxAlt[big])
			tiny := size[small] < areaMergeMaxSize || maxAlt[small] < areaMergeMaxAltitude
			if tiny || (elev[small] == elev[big] && a/lowPeak >= areaMergeAltRatio) {
				parent[small] = big
				size[big] += size[small]
				maxAlt[big] = max(maxAlt[big], maxAlt[small])
				cur = big
				continue
			}
			frontier = append(frontier, frontierCell{Idx: i, A: cur, B: other})
		}
		cur = find(cur)
		label[i] = cur
		size[cur]++
	}

	dense := make(map[int32]int32)
	area := make([]int32, w*h)
	for i := range area {
		if label[i] < 0 {
			area[i] = -1
			continue
		}
		r := find(label[i])
		id, ok := dense[r]
		if !ok {
			id = int32(len(dense))
			dense[r] = id
		}
		area[i] = id
	}
	resolved := frontier[:0]
	for _, f := range frontier {
		a, b := dense[find(f.A)], dense[find(f.B)]
		if a == b {
			continue
		}
		if a > b {
			a, b = b, a
		}
		resolved = append(resolved, frontierCell{Idx: f.Idx, A: a, B: b})
	}
	return &terrainAreas{
		Width:     w,
		Height:    h,
		Passable:  passable,
		Ramp:      rampMask(data),
		Elevation: elevation,
		Altitude:  alt,
		Area:      area,
		AreaCount: len(dense),
		Frontier:  resolved,
	}
}
