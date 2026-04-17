package basedetect

import (
	"fmt"
	"math"
	"sort"

	"github.com/marianogappa/scmapanalyzer/internal/model"
)

const radiusSafety = 0.98

type point struct {
	X float64
	Y float64
}

type mstEdge struct {
	A int
	B int
	W float64
}

func DetectBases(widthTiles int, heightTiles int, minerals []model.MapResource, geysers []model.MapResource, starts []model.StartLocation) []model.Base {
	points := make([]point, 0, len(minerals)+len(geysers))
	for _, m := range minerals {
		points = append(points, point{X: float64(m.X), Y: float64(m.Y)})
	}
	for _, g := range geysers {
		points = append(points, point{X: float64(g.X), Y: float64(g.Y)})
	}
	if len(points) == 0 {
		return nil
	}

	_, _, _, _, labels := chooseMSTLabels(points)
	bases := makeBases(points, labels)
	if len(bases) == 0 {
		return nil
	}

	for _, sl := range starts {
		idx := nearestBase(float64(sl.X), float64(sl.Y), bases)
		if idx < 0 {
			continue
		}
		bases[idx].StartCount++
		bases[idx].IsStarting = true
	}

	assignPerBaseRadii(bases, radiusSafety)
	enlargeStartBaseRadii(bases, radiusSafety)

	for i := range bases {
		oc := calculateStartLocationOclock(
			widthTiles,
			heightTiles,
			int(math.Round(bases[i].CenterX)),
			int(math.Round(bases[i].CenterY)),
		)
		if bases[i].IsStarting {
			bases[i].DisplayName = fmt.Sprintf("at %d", oc)
		} else {
			bases[i].DisplayName = fmt.Sprintf("an expa near %d", oc)
		}
	}
	return bases
}

func chooseMSTLabels(points []point) (float64, float64, int, float64, []int) {
	bestAlpha := 1.9
	bestBeta := 2.3
	bestK := 0
	bestLabels := []int{}
	bestSil := -1.0
	bestScore := -math.MaxFloat64

	alphas := []float64{1.5, 1.7, 1.9, 2.1, 2.3}
	betas := []float64{2.0, 2.3, 2.6, 2.9}
	for _, alpha := range alphas {
		for _, beta := range betas {
			labels, k := labelsFromMSTCuts(points, 3, alpha, beta)
			if k < 4 {
				continue
			}
			sil := silhouetteScore(points, labels, k)
			score := sil
			sizes := clusterSizes(labels, k)
			for _, size := range sizes {
				if size < 5 {
					score -= 0.04 * float64(5-size)
				}
				if size > 22 {
					score -= 0.03 * float64(size-22)
				}
			}
			if k < 8 {
				score -= 0.06 * float64(8-k)
			}
			if k > 24 {
				score -= 0.04 * float64(k-24)
			}
			if score > bestScore {
				bestScore = score
				bestSil = sil
				bestAlpha = alpha
				bestBeta = beta
				bestK = k
				bestLabels = labels
			}
		}
	}
	if bestK == 0 {
		labels, k := labelsFromMSTCuts(points, 3, 1.9, 2.3)
		return 1.9, 2.3, k, silhouetteScore(points, labels, maxInt(k, 1)), labels
	}
	return bestAlpha, bestBeta, bestK, bestSil, bestLabels
}

func labelsFromMSTCuts(points []point, kNN int, alpha float64, beta float64) ([]int, int) {
	n := len(points)
	if n == 0 {
		return []int{}, 0
	}
	if n == 1 {
		return []int{0}, 1
	}
	localScale := kthNeighborDistances(points, kNN)
	medianScale := percentile(localScale, 0.5)
	mst := primMST(points)

	uf := newUnionFind(n)
	for _, e := range mst {
		localThreshold := alpha * math.Max(localScale[e.A], localScale[e.B])
		globalThreshold := beta * medianScale
		if e.W <= localThreshold && e.W <= globalThreshold {
			uf.union(e.A, e.B)
		}
	}

	components := map[int][]int{}
	for i := 0; i < n; i++ {
		root := uf.find(i)
		components[root] = append(components[root], i)
	}

	const minComponentSize = 4
	bigRoots := make([]int, 0, len(components))
	for root, members := range components {
		if len(members) >= minComponentSize {
			bigRoots = append(bigRoots, root)
		}
	}
	sort.Ints(bigRoots)
	if len(bigRoots) == 0 {
		for root := range components {
			bigRoots = append(bigRoots, root)
		}
		sort.Ints(bigRoots)
	}

	rootCenters := map[int][2]float64{}
	for _, root := range bigRoots {
		rootCenters[root] = centroid(points, components[root])
	}
	pointRoot := make([]int, n)
	for i := 0; i < n; i++ {
		pointRoot[i] = uf.find(i)
	}
	for _, members := range components {
		if len(members) >= minComponentSize || len(bigRoots) == 0 {
			continue
		}
		targetRoot := bigRoots[0]
		best := math.MaxFloat64
		for _, bRoot := range bigRoots {
			c := rootCenters[bRoot]
			d := averageDistanceToPoint(points, members, c[0], c[1])
			if d < best {
				best = d
				targetRoot = bRoot
			}
		}
		for _, idx := range members {
			pointRoot[idx] = targetRoot
		}
	}

	labelByRoot := map[int]int{}
	labels := make([]int, n)
	next := 0
	for i := 0; i < n; i++ {
		root := pointRoot[i]
		lbl, ok := labelByRoot[root]
		if !ok {
			lbl = next
			labelByRoot[root] = lbl
			next++
		}
		labels[i] = lbl
	}
	return labels, next
}

func makeBases(points []point, labels []int) []model.Base {
	per := map[int][]int{}
	for i, l := range labels {
		if l < 0 {
			continue
		}
		per[l] = append(per[l], i)
	}
	ids := make([]int, 0, len(per))
	for id := range per {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	out := make([]model.Base, 0, len(ids))
	for _, id := range ids {
		members := per[id]
		if len(members) < 4 {
			continue
		}
		c := centroid(points, members)
		natural := 0.0
		for _, mi := range members {
			d := dist(c[0], c[1], points[mi].X, points[mi].Y)
			if d > natural {
				natural = d
			}
		}
		out = append(out, model.Base{
			CenterX:       c[0],
			CenterY:       c[1],
			NaturalRadius: natural,
		})
	}
	return out
}

func assignPerBaseRadii(bases []model.Base, safety float64) {
	for i := range bases {
		minHalfDist := math.MaxFloat64
		for j := range bases {
			if i == j {
				continue
			}
			d := dist(bases[i].CenterX, bases[i].CenterY, bases[j].CenterX, bases[j].CenterY)
			if d/2 < minHalfDist {
				minHalfDist = d / 2
			}
		}
		if len(bases) == 1 {
			minHalfDist = bases[i].NaturalRadius
		}
		capR := minHalfDist * safety
		if bases[i].NaturalRadius < capR {
			bases[i].GeoRadius = bases[i].NaturalRadius
		} else {
			bases[i].GeoRadius = capR
		}
	}
}

func enlargeStartBaseRadii(bases []model.Base, safety float64) {
	startIdx := make([]int, 0, len(bases))
	for i, b := range bases {
		if b.StartCount > 0 {
			startIdx = append(startIdx, i)
		}
	}
	if len(startIdx) == 0 {
		return
	}
	sort.Ints(startIdx)
	steps := []float64{64, 16, 4, 1, 0.25}
	for _, step := range steps {
		for turns := 0; turns < 20000; turns++ {
			progress := false
			for _, i := range startIdx {
				if canGrowBaseRadius(bases, i, step, safety) {
					bases[i].GeoRadius += step
					progress = true
				}
			}
			if !progress {
				break
			}
		}
	}
}

func canGrowBaseRadius(bases []model.Base, idx int, step float64, safety float64) bool {
	newR := bases[idx].GeoRadius + step
	for j := range bases {
		if j == idx {
			continue
		}
		d := dist(bases[idx].CenterX, bases[idx].CenterY, bases[j].CenterX, bases[j].CenterY)
		if newR+bases[j].GeoRadius > d*safety {
			return false
		}
	}
	return true
}

func nearestBase(x float64, y float64, bases []model.Base) int {
	if len(bases) == 0 {
		return -1
	}
	best := 0
	bestD := math.MaxFloat64
	for i, b := range bases {
		d := dist(x, y, b.CenterX, b.CenterY)
		if d < bestD {
			bestD = d
			best = i
		}
	}
	return best
}

func dist(x1 float64, y1 float64, x2 float64, y2 float64) float64 {
	dx := x1 - x2
	dy := y1 - y2
	return math.Sqrt(dx*dx + dy*dy)
}

func kthNeighborDistances(points []point, k int) []float64 {
	n := len(points)
	if n == 0 {
		return []float64{}
	}
	if k < 1 {
		k = 1
	}
	if k >= n {
		k = n - 1
	}
	res := make([]float64, n)
	for i := 0; i < n; i++ {
		ds := make([]float64, 0, n-1)
		for j := 0; j < n; j++ {
			if i == j {
				continue
			}
			ds = append(ds, dist(points[i].X, points[i].Y, points[j].X, points[j].Y))
		}
		sort.Float64s(ds)
		res[i] = ds[k-1]
	}
	return res
}

func primMST(points []point) []mstEdge {
	n := len(points)
	if n <= 1 {
		return []mstEdge{}
	}
	inTree := make([]bool, n)
	minDist := make([]float64, n)
	parent := make([]int, n)
	for i := 0; i < n; i++ {
		minDist[i] = math.MaxFloat64
		parent[i] = -1
	}
	minDist[0] = 0
	edges := make([]mstEdge, 0, n-1)
	for step := 0; step < n; step++ {
		u := -1
		best := math.MaxFloat64
		for i := 0; i < n; i++ {
			if !inTree[i] && minDist[i] < best {
				best = minDist[i]
				u = i
			}
		}
		if u == -1 {
			break
		}
		inTree[u] = true
		if parent[u] >= 0 {
			edges = append(edges, mstEdge{A: parent[u], B: u, W: best})
		}
		for v := 0; v < n; v++ {
			if inTree[v] || u == v {
				continue
			}
			w := dist(points[u].X, points[u].Y, points[v].X, points[v].Y)
			if w < minDist[v] {
				minDist[v] = w
				parent[v] = u
			}
		}
	}
	return edges
}

func silhouetteScore(points []point, labels []int, k int) float64 {
	clusters := make([][]int, k)
	for i, l := range labels {
		clusters[l] = append(clusters[l], i)
	}
	if len(points) == 0 {
		return 0
	}
	total := 0.0
	for i := range points {
		my := labels[i]
		a := 0.0
		if len(clusters[my]) > 1 {
			for _, j := range clusters[my] {
				if i == j {
					continue
				}
				a += dist(points[i].X, points[i].Y, points[j].X, points[j].Y)
			}
			a /= float64(len(clusters[my]) - 1)
		}
		b := math.MaxFloat64
		for c := 0; c < k; c++ {
			if c == my || len(clusters[c]) == 0 {
				continue
			}
			avg := 0.0
			for _, j := range clusters[c] {
				avg += dist(points[i].X, points[i].Y, points[j].X, points[j].Y)
			}
			avg /= float64(len(clusters[c]))
			if avg < b {
				b = avg
			}
		}
		if b == math.MaxFloat64 {
			continue
		}
		den := math.Max(a, b)
		if den == 0 {
			continue
		}
		total += (b - a) / den
	}
	return total / float64(len(points))
}

func clusterSizes(labels []int, k int) []int {
	sizes := make([]int, k)
	for _, l := range labels {
		sizes[l]++
	}
	return sizes
}

func centroid(points []point, members []int) [2]float64 {
	sx := 0.0
	sy := 0.0
	for _, mi := range members {
		sx += points[mi].X
		sy += points[mi].Y
	}
	return [2]float64{sx / float64(len(members)), sy / float64(len(members))}
}

func averageDistanceToPoint(points []point, members []int, x float64, y float64) float64 {
	if len(members) == 0 {
		return math.MaxFloat64
	}
	total := 0.0
	for _, mi := range members {
		total += dist(points[mi].X, points[mi].Y, x, y)
	}
	return total / float64(len(members))
}

func percentile(vals []float64, p float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	x := make([]float64, len(vals))
	copy(x, vals)
	sort.Float64s(x)
	if p <= 0 {
		return x[0]
	}
	if p >= 1 {
		return x[len(x)-1]
	}
	pos := p * float64(len(x)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return x[lo]
	}
	frac := pos - float64(lo)
	return x[lo]*(1-frac) + x[hi]*frac
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

type unionFind struct {
	parent []int
	rank   []int
}

func newUnionFind(n int) *unionFind {
	p := make([]int, n)
	r := make([]int, n)
	for i := 0; i < n; i++ {
		p[i] = i
	}
	return &unionFind{parent: p, rank: r}
}

func (u *unionFind) find(x int) int {
	if u.parent[x] != x {
		u.parent[x] = u.find(u.parent[x])
	}
	return u.parent[x]
}

func (u *unionFind) union(a int, b int) {
	ra := u.find(a)
	rb := u.find(b)
	if ra == rb {
		return
	}
	if u.rank[ra] < u.rank[rb] {
		u.parent[ra] = rb
		return
	}
	if u.rank[ra] > u.rank[rb] {
		u.parent[rb] = ra
		return
	}
	u.parent[rb] = ra
	u.rank[ra]++
}

func calculateStartLocationOclock(tileX int, tileY int, startLocationX int, startLocationY int) int {
	mapWidth := tileX * 32
	mapHeight := tileY * 32

	centerX := float64(mapWidth) / 2.0
	centerY := float64(mapHeight) / 2.0

	relX := float64(startLocationX) - centerX
	relY := float64(startLocationY) - centerY
	angle := math.Atan2(relY, relX)
	angleDegrees := angle * 180.0 / math.Pi
	if angleDegrees < 0 {
		angleDegrees += 360
	}
	switch {
	case angleDegrees >= 337.5 || angleDegrees < 22.5:
		return 3
	case angleDegrees >= 22.5 && angleDegrees < 67.5:
		return 5
	case angleDegrees >= 67.5 && angleDegrees < 112.5:
		return 6
	case angleDegrees >= 112.5 && angleDegrees < 157.5:
		return 7
	case angleDegrees >= 157.5 && angleDegrees < 202.5:
		return 9
	case angleDegrees >= 202.5 && angleDegrees < 247.5:
		return 11
	case angleDegrees >= 247.5 && angleDegrees < 292.5:
		return 12
	case angleDegrees >= 292.5 && angleDegrees < 337.5:
		return 1
	default:
		return 3
	}
}
