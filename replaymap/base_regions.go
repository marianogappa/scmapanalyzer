package replaymap

import (
	"container/heap"
	"math"
	"sort"
)

// Geodesic reach caps (minitiles) applied when a base sits in an area that is
// much larger than the base itself, e.g. an expansion in the open middle.
// Mains are effectively uncapped: their whole plateau is the base.
const (
	startReachCap = 200
	expaReachCap  = 52
	// chokeMaxWidth discards frontiers wider than this (minitiles): such a
	// boundary is open ground, not something a contain can hold.
	chokeMaxWidth = 64
	// plateauExitMinCells ignores walkable specks touching a plateau's cliff
	// edge; a real ramp mouth is several minitiles wide.
	plateauExitMinCells = 4
	// chokeSideReach is how far (minitiles) past a choke we look for the base
	// on the other side, e.g. a natural whose area stops short of the ramp foot.
	chokeSideReach = 24
	// chokeChordDirections is how many directions are tried when looking for
	// the shortest wall-to-wall crossing through a choke's saddle.
	chokeChordDirections = 36
	// chokeDedupeDist (minitiles, Manhattan) merges chokes a base reports twice.
	chokeDedupeDist = 8
	// sideRoomOpeningRatio: a baseless room joins a main only when its opening
	// into the main is this much wider than any other opening it has.
	sideRoomOpeningRatio = 1.5
)

type baseSeed struct {
	Center point
	Start  bool
}

type baseRegions struct {
	Masks  [][]bool
	Chokes [][]chokeSegment
}

type chokeSegment struct {
	A, B      TilePoint
	Center    TilePoint
	Width     float64
	OtherBase int
}

// buildBaseRegions gives every base the terrain it stands on:
//   - each base grows over its own watershed area (split by distance when bases
//     share one), capped in reach so a base in open ground stays local;
//   - a base on its own raised plateau or island takes all of it; a main on a
//     flat map instead absorbs the baseless rooms it is most widely connected to;
//   - chokes are a plateau's exits (its ramps), plus watershed frontiers
//     crossing a base's edge.
func buildBaseRegions(ta *terrainAreas, seeds []baseSeed) *baseRegions {
	w, h := ta.Width, ta.Height
	n := len(seeds)
	seedArea := make([]int32, n)
	seedIdx := make([]int, n)
	for i, s := range seeds {
		seedArea[i], seedIdx[i] = -1, -1
		if idx, ok := nearestPassable(ta, s.Center); ok {
			seedArea[i], seedIdx[i] = ta.Area[idx], idx
		}
	}
	owner := growWithinAreas(ta, seeds, seedIdx, seedArea)
	chokes := extractChokes(ta)

	plateau := make([]bool, n)
	for i, s := range seeds {
		if seedIdx[i] < 0 {
			continue
		}
		plateau[i] = claimPlateau(ta, owner, seedIdx, i)
		if !plateau[i] && s.Start {
			absorbSideRooms(ta, owner, chokes, seedArea, i)
		}
	}

	masks := make([][]bool, n)
	for i := range masks {
		masks[i] = make([]bool, w*h)
	}
	for idx, o := range owner {
		if o >= 0 {
			masks[o][idx] = true
		}
	}

	out := make([][]chokeSegment, n)
	add := func(seg chokeSegment, in, other int32) {
		seg.OtherBase = int(other)
		out[in] = append(out[in], seg)
	}
	for _, c := range chokes {
		if c.seg.Width > chokeMaxWidth {
			continue
		}
		in, other, ok := chokeSides(ta, owner, c.seg)
		if !ok || in == other {
			continue
		}
		for _, side := range [2][2]int32{{in, other}, {other, in}} {
			if side[0] >= 0 && !plateau[side[0]] && (side[1] < 0 || !plateau[side[1]]) {
				add(c.seg, side[0], side[1])
			}
		}
	}
	for i := range seeds {
		if !plateau[i] {
			continue
		}
		for _, exit := range plateauExits(ta, masks[i]) {
			other := baseBeyond(ta, owner, masks[i], exit.cells)
			add(exit.seg, int32(i), other)
			if other >= 0 && !plateau[other] {
				add(exit.seg, other, int32(i))
			}
		}
	}
	for i := range out {
		out[i] = dedupeChokes(out[i])
	}
	return &baseRegions{Masks: masks, Chokes: out}
}

// baseBeyond walks outward from a plateau exit, away from the plateau, and
// returns the nearest base whose ground it reaches (-1 if none is close).
func baseBeyond(ta *terrainAreas, owner []int32, mask []bool, exit []int) int32 {
	w, h := ta.Width, ta.Height
	dist := make(map[int]int, len(exit))
	queue := append([]int{}, exit...)
	for _, c := range exit {
		dist[c] = 0
	}
	for q := 0; q < len(queue); q++ {
		c := queue[q]
		if o := owner[c]; o >= 0 && !mask[c] {
			return o
		}
		if dist[c] >= chokeSideReach {
			continue
		}
		x, y := c%w, c/w
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			nx, ny := x+d[0], y+d[1]
			if nx < 0 || ny < 0 || nx >= w || ny >= h {
				continue
			}
			ni := ny*w + nx
			if _, seen := dist[ni]; seen || mask[ni] || !ta.Passable[ni] {
				continue
			}
			dist[ni] = dist[c] + 1
			queue = append(queue, ni)
		}
	}
	return -1
}

// dedupeChokes drops a choke that repeats an earlier one at nearly the same
// spot, as when a main and its natural both report the ramp between them.
func dedupeChokes(segs []chokeSegment) []chokeSegment {
	out := segs[:0]
	for _, s := range segs {
		dup := false
		for _, k := range out {
			if abs(s.Center.X-k.Center.X)+abs(s.Center.Y-k.Center.Y) <= chokeDedupeDist {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, s)
		}
	}
	return out
}

// growWithinAreas runs a multi-source Dijkstra (8-neighbour, Euclidean-ish
// steps) from every base, confined to the base's own watershed area and capped
// in reach. Diagonal steps need both orthogonal cells open, so growth never
// slips through a one-cell gap between cliffs.
func growWithinAreas(ta *terrainAreas, seeds []baseSeed, seedIdx []int, seedArea []int32) []int32 {
	w, h := ta.Width, ta.Height
	owner := make([]int32, w*h)
	dist := make([]float32, w*h)
	for i := range owner {
		owner[i] = -1
		dist[i] = math.MaxFloat32
	}
	pq := &growNodeHeap{}
	for i, idx := range seedIdx {
		if idx < 0 || dist[idx] == 0 {
			continue
		}
		dist[idx] = 0
		owner[idx] = int32(i)
		heap.Push(pq, growNode{Cost: 0, Owner: i, Idx: idx})
	}
	steps := [8]struct {
		dx, dy int
		c      float32
	}{{1, 0, 1}, {-1, 0, 1}, {0, 1, 1}, {0, -1, 1}, {1, 1, math.Sqrt2}, {1, -1, math.Sqrt2}, {-1, 1, math.Sqrt2}, {-1, -1, math.Sqrt2}}
	for pq.Len() > 0 {
		cur := heap.Pop(pq).(growNode)
		if float32(cur.Cost) > dist[cur.Idx] || owner[cur.Idx] != int32(cur.Owner) {
			continue
		}
		reach := float32(expaReachCap)
		if seeds[cur.Owner].Start {
			reach = startReachCap
		}
		area := seedArea[cur.Owner]
		x, y := cur.Idx%w, cur.Idx/w
		inArea := func(x, y int) bool { return x >= 0 && y >= 0 && x < w && y < h && ta.Area[y*w+x] == area }
		for _, s := range steps {
			nx, ny := x+s.dx, y+s.dy
			if !inArea(nx, ny) || (s.dx != 0 && s.dy != 0 && (!inArea(x+s.dx, y) || !inArea(x, y+s.dy))) {
				continue
			}
			nd := float32(cur.Cost) + s.c
			ni := ny*w + nx
			if nd > reach || nd >= dist[ni] {
				continue
			}
			dist[ni] = nd
			owner[ni] = int32(cur.Owner)
			heap.Push(pq, growNode{Cost: float64(nd), Owner: cur.Owner, Idx: ni})
		}
	}
	return owner
}

// absorbSideRooms gives a main the baseless watershed rooms whose widest
// opening leads into that main: on flat maps the watershed can split a main
// at an internal narrowing, leaving the ramp-side half orphaned.
func absorbSideRooms(ta *terrainAreas, owner []int32, chokes []areaChoke, seedArea []int32, base int) {
	main := seedArea[base]
	hosts := make(map[int32]bool)
	for _, a := range seedArea {
		hosts[a] = true
	}
	widest := make(map[[2]int32]float64)
	for _, c := range chokes {
		k := [2]int32{c.areaA, c.areaB}
		widest[k] = max(widest[k], c.seg.Width)
	}
	size := make(map[int32]int)
	for _, a := range ta.Area {
		size[a]++
	}
	claim := make(map[int32]bool)
	for k, wMain := range widest {
		var room int32
		switch main {
		case k[0]:
			room = k[1]
		case k[1]:
			room = k[0]
		default:
			continue
		}
		if hosts[room] || size[room] > 2*size[main] {
			continue
		}
		best := true
		for k2, w2 := range widest {
			if k2 != k && (k2[0] == room || k2[1] == room) && w2*sideRoomOpeningRatio > wMain {
				best = false
			}
		}
		if best {
			claim[room] = true
		}
	}
	for i, a := range ta.Area {
		if claim[a] && owner[i] < 0 {
			owner[i] = int32(base)
		}
	}
}

// plateauExits finds where walkable ground leaves a base's plateau (its
// ramps, in practice) and fits a choke crossing at each exit.
type plateauExit struct {
	seg   chokeSegment
	cells []int
}

func plateauExits(ta *terrainAreas, mask []bool) []plateauExit {
	w, h := ta.Width, ta.Height
	var exits []int
	for i, in := range mask {
		if in || !ta.Passable[i] {
			continue
		}
		x, y := i%w, i/w
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			nx, ny := x+d[0], y+d[1]
			if nx >= 0 && ny >= 0 && nx < w && ny < h && mask[ny*w+nx] {
				exits = append(exits, i)
				break
			}
		}
	}
	var out []plateauExit
	for _, cluster := range clusterCells(w, exits, 2) {
		if len(cluster) < plateauExitMinCells {
			continue
		}
		if seg, ok := fitChoke(ta, cluster); ok && seg.Width <= chokeMaxWidth {
			out = append(out, plateauExit{seg: seg, cells: cluster})
		}
	}
	return out
}

// claimPlateau extends a base over its whole ground-level plateau (the
// same-elevation, non-ramp terrain connected to it): a main, a secluded
// high-ground expansion reached by ramp, or an island. Skipped when the plateau
// holds another base or dwarfs the base's own room, which is what keeps bases
// in shared open ground (and mains on flat maps) to their watershed room.
func claimPlateau(ta *terrainAreas, owner []int32, seedIdx []int, base int) bool {
	w, h := ta.Width, ta.Height
	start := seedIdx[base]
	level := ta.Elevation[start]
	inPlateau := func(i int) bool { return ta.Passable[i] && !ta.Ramp[i] && ta.Elevation[i] == level }
	if !inPlateau(start) {
		return false
	}
	seen := make(map[int]bool)
	queue := []int{start}
	seen[start] = true
	for q := 0; q < len(queue); q++ {
		x, y := queue[q]%w, queue[q]/w
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			nx, ny := x+d[0], y+d[1]
			if nx < 0 || ny < 0 || nx >= w || ny >= h {
				continue
			}
			ni := ny*w + nx
			if !seen[ni] && inPlateau(ni) {
				seen[ni] = true
				queue = append(queue, ni)
			}
		}
	}
	for j, idx := range seedIdx {
		if j != base && idx >= 0 && seen[idx] {
			return false
		}
	}
	own := 0
	for _, o := range owner {
		if o == int32(base) {
			own++
		}
	}
	if len(queue) > 3*own {
		return false
	}
	for _, idx := range queue {
		owner[idx] = int32(base)
	}
	return true
}

// chokeSides samples the ground a few minitiles to either side of a choke
// crossing and reports which base owns each side (-1 for none).
func chokeSides(ta *terrainAreas, owner []int32, seg chokeSegment) (int32, int32, bool) {
	w, h := ta.Width, ta.Height
	dx, dy := float64(seg.B.X-seg.A.X), float64(seg.B.Y-seg.A.Y)
	l := math.Hypot(dx, dy)
	if l == 0 {
		return 0, 0, false
	}
	nx, ny := -dy/l, dx/l
	cx, cy := float64(seg.Center.X)+0.5, float64(seg.Center.Y)+0.5
	side := func(sign float64) int32 {
		walked := false
		for t := 3.0; t <= chokeSideReach; t++ {
			x := int(math.Floor(cx + sign*nx*t))
			y := int(math.Floor(cy + sign*ny*t))
			if x < 0 || y < 0 || x >= w || y >= h || !ta.Passable[y*w+x] {
				if walked {
					return -1
				}
				continue
			}
			walked = true
			if o := owner[y*w+x]; o >= 0 {
				return o
			}
		}
		return -1
	}
	return side(1), side(-1), true
}

func nearestPassable(ta *terrainAreas, c point) (int, bool) {
	x0 := clampInt(int(c.X), 0, ta.Width-1)
	y0 := clampInt(int(c.Y), 0, ta.Height-1)
	for r := 0; r <= passableSeedSearchRadius; r++ {
		for y := y0 - r; y <= y0+r; y++ {
			for x := x0 - r; x <= x0+r; x++ {
				if x < 0 || y < 0 || x >= ta.Width || y >= ta.Height {
					continue
				}
				if max(abs(x-x0), abs(y-y0)) != r {
					continue
				}
				if idx := y*ta.Width + x; ta.Area[idx] >= 0 {
					return idx, true
				}
			}
		}
	}
	return 0, false
}

type areaChoke struct {
	areaA, areaB int32
	seg          chokeSegment
}

// extractChokes groups frontier cells by the pair of areas they separate,
// splits each group into spatial clusters (one per physical passage) and fits
// a wall-to-wall segment across each.
func extractChokes(ta *terrainAreas) []areaChoke {
	w := ta.Width
	byPair := make(map[[2]int32][]int)
	for _, f := range ta.Frontier {
		k := [2]int32{f.A, f.B}
		byPair[k] = append(byPair[k], f.Idx)
	}
	keys := make([][2]int32, 0, len(byPair))
	for k := range byPair {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i][0] != keys[j][0] {
			return keys[i][0] < keys[j][0]
		}
		return keys[i][1] < keys[j][1]
	})
	var out []areaChoke
	for _, k := range keys {
		for _, cluster := range clusterCells(w, byPair[k], 3) {
			if seg, ok := fitChoke(ta, cluster); ok {
				out = append(out, areaChoke{areaA: k[0], areaB: k[1], seg: seg})
			}
		}
	}
	return out
}

func clusterCells(w int, cells []int, gap int) [][]int {
	seen := make([]bool, len(cells))
	var out [][]int
	for i := range cells {
		if seen[i] {
			continue
		}
		seen[i] = true
		group := []int{cells[i]}
		for q := 0; q < len(group); q++ {
			gx, gy := group[q]%w, group[q]/w
			for j := range cells {
				if seen[j] {
					continue
				}
				if max(abs(cells[j]%w-gx), abs(cells[j]/w-gy)) <= gap {
					seen[j] = true
					group = append(group, cells[j])
				}
			}
		}
		out = append(out, group)
	}
	return out
}

// fitChoke crosses the passage at its narrowest frontier cell (the saddle,
// highest altitude on the frontier) along the shortest wall-to-wall chord
// through it.
func fitChoke(ta *terrainAreas, cells []int) (chokeSegment, bool) {
	w, h := ta.Width, ta.Height
	saddle := cells[0]
	for _, c := range cells {
		if ta.Altitude[c] > ta.Altitude[saddle] {
			saddle = c
		}
	}
	sx, sy := float64(saddle%w)+0.5, float64(saddle/w)+0.5
	blocked := func(x, y int) bool { return x < 0 || y < 0 || x >= w || y >= h || !ta.Passable[y*w+x] }
	march := func(dx, dy float64) float64 {
		t := 0.0
		for ; t < chokeMaxWidth; t += 0.5 {
			if blocked(int(math.Floor(sx+dx*(t+0.5))), int(math.Floor(sy+dy*(t+0.5)))) {
				break
			}
		}
		return t
	}
	best, bestTheta, bestLo, bestHi := math.MaxFloat64, 0.0, 0.0, 0.0
	for k := 0; k < chokeChordDirections; k++ {
		theta := math.Pi * float64(k) / chokeChordDirections
		ux, uy := math.Cos(theta), math.Sin(theta)
		lo, hi := march(-ux, -uy), march(ux, uy)
		if lo+hi < best {
			best, bestTheta, bestLo, bestHi = lo+hi, theta, lo, hi
		}
	}
	ux, uy := math.Cos(bestTheta), math.Sin(bestTheta)
	tp := func(t float64) TilePoint {
		return TilePoint{X: int(math.Floor(sx + ux*t)), Y: int(math.Floor(sy + uy*t))}
	}
	return chokeSegment{
		A:      tp(-bestLo),
		B:      tp(bestHi),
		Center: TilePoint{X: saddle % w, Y: saddle / w},
		Width:  best,
	}, best > 0
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

type growNode struct {
	Cost  float64
	Owner int
	Idx   int
}

type growNodeHeap []growNode

func (h growNodeHeap) Len() int               { return len(h) }
func (h growNodeHeap) Less(i int, j int) bool { return h[i].Cost < h[j].Cost }
func (h growNodeHeap) Swap(i int, j int)      { h[i], h[j] = h[j], h[i] }
func (h *growNodeHeap) Push(x any)            { *h = append(*h, x.(growNode)) }
func (h *growNodeHeap) Pop() any {
	old := *h
	n := len(old)
	v := old[n-1]
	*h = old[:n-1]
	return v
}
