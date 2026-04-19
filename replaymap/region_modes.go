package replaymap

import (
	"container/heap"
	"errors"
	"math"
)

type buildRegionMasksInput struct {
	Width           int
	Height          int
	DistWR          []int
	DistEdge        []int
	DistWallOnly    []int
	WallRampBarrier []bool
	WallOnlyBarrier []bool
	StartOwners     []int
	ExpaOwners      []int
	StartCenters    []point
	ExpaCenters     []point
	RampForbidden   []bool
}

type buildRegionMasksOutput struct {
	StartMasks [][]bool
	ExpaMasks  [][]bool
}

func buildRegionMasks(in buildRegionMasksInput) (*buildRegionMasksOutput, error) {
	return buildMasksWallBasin(in, wallBasinProfileDefault())
}

type wallBasinProfile struct {
	StartPreCapMul   float64
	StartFinalCapMul float64
	StartWallW       float64
	StartEdgeW       float64
	StartChokeW      float64
	StartCenterW     float64

	ExpaPreCapMul    float64
	ExpaFinalCapMul  float64
	ExpaWallW        float64
	ExpaEdgeW        float64
	ExpaChokeW       float64
	ExpaCenterW      float64
	ExpaMinNeighbors int
}

func wallBasinProfileDefault() wallBasinProfile {
	// Starts aggressively consume wall-delimited space; expansions are shape-biased
	// and avoid drifting across long passages.
	return wallBasinProfile{
		StartPreCapMul:   3.3,
		StartFinalCapMul: 1.42,
		StartWallW:       1.8,
		StartEdgeW:       0.6,
		StartChokeW:      0.12,
		StartCenterW:     0.006,

		ExpaPreCapMul:    1.6,
		ExpaFinalCapMul:  0.9,
		ExpaWallW:        4.0,
		ExpaEdgeW:        2.0,
		ExpaChokeW:       1.25,
		ExpaCenterW:      0.12,
		ExpaMinNeighbors: 2,
	}
}

func buildMasksWallBasin(in buildRegionMasksInput, p wallBasinProfile) (*buildRegionMasksOutput, error) {
	startPreCaps := estimateEqualCaps(
		in.Width,
		in.Height,
		in.StartOwners,
		in.DistWallOnly,
		in.DistEdge,
		in.StartCenters,
		in.RampForbidden,
	)
	if len(startPreCaps) == 0 {
		return nil, errors.New("invalid starting area")
	}
	preCaps := make([]int, len(startPreCaps))
	for i := range preCaps {
		preCaps[i] = max(int(math.Round(float64(startPreCaps[i])*p.StartPreCapMul)), 1)
	}
	startOpen := walkableNeighborCounts(in.Width, in.Height, in.WallOnlyBarrier)
	startMasksPass1, _ := growCompetitiveByCost(
		in.Width,
		in.Height,
		in.StartCenters,
		preCaps,
		in.WallOnlyBarrier,
		func(owner int, idx int, _ int, _ int) bool {
			return in.StartOwners[idx] == owner &&
				in.DistWallOnly[idx] > startClearanceTiles &&
				in.DistEdge[idx] > startEdgeClearance &&
				!in.RampForbidden[idx]
		},
		func(owner int, idx int, x int, y int) float64 {
			return 1.0 +
				p.StartWallW/(1.0+float64(in.DistWR[idx])) +
				p.StartEdgeW/(1.0+float64(in.DistEdge[idx])) +
				p.StartChokeW*narrowPassagePenalty(startOpen[idx]) +
				p.StartCenterW*distanceToCenterPenalty(in.StartCenters[owner], x, y)
		},
	)
	startTarget := max(int(math.Round(float64(max(minPositive(regionAreas(startMasksPass1)), 1))*p.StartFinalCapMul)), 1)
	startCaps := make([]int, len(startPreCaps))
	for i := range startCaps {
		startCaps[i] = startTarget
	}
	startMasks, _ := growCompetitiveByCost(
		in.Width,
		in.Height,
		in.StartCenters,
		startCaps,
		in.WallOnlyBarrier,
		func(owner int, idx int, _ int, _ int) bool {
			return in.StartOwners[idx] == owner &&
				in.DistWallOnly[idx] > startClearanceTiles &&
				in.DistEdge[idx] > startEdgeClearance &&
				!in.RampForbidden[idx]
		},
		func(owner int, idx int, x int, y int) float64 {
			return 1.0 +
				p.StartWallW/(1.0+float64(in.DistWR[idx])) +
				p.StartEdgeW/(1.0+float64(in.DistEdge[idx])) +
				p.StartChokeW*narrowPassagePenalty(startOpen[idx]) +
				p.StartCenterW*distanceToCenterPenalty(in.StartCenters[owner], x, y)
		},
	)

	startOccupied := combineMasks(in.Width*in.Height, startMasks)
	expaPreCaps := make([]int, len(in.ExpaCenters))
	for i := range expaPreCaps {
		expaPreCaps[i] = max(int(math.Round(float64(startTarget)*p.ExpaPreCapMul)), 1)
	}
	blockedExpas := make([]bool, len(startOccupied))
	for i := range blockedExpas {
		blockedExpas[i] = in.WallRampBarrier[i] || startOccupied[i]
	}
	expaOpen := walkableNeighborCounts(in.Width, in.Height, blockedExpas)
	expaMasksPass1, _ := growCompetitiveByCost(
		in.Width,
		in.Height,
		in.ExpaCenters,
		expaPreCaps,
		blockedExpas,
		func(owner int, idx int, _ int, _ int) bool {
			return in.ExpaOwners[idx] == owner &&
				in.DistWR[idx] > startClearanceTiles &&
				in.DistEdge[idx] > startEdgeClearance &&
				expaOpen[idx] >= p.ExpaMinNeighbors
		},
		func(owner int, idx int, x int, y int) float64 {
			return 1.0 +
				p.ExpaWallW/(1.0+float64(in.DistWR[idx])) +
				p.ExpaEdgeW/(1.0+float64(in.DistEdge[idx])) +
				p.ExpaChokeW*narrowPassagePenalty(expaOpen[idx]) +
				p.ExpaCenterW*distanceToCenterPenalty(in.ExpaCenters[owner], x, y)
		},
	)
	expaTarget := max(minPositive(regionAreas(expaMasksPass1)), 1)
	expaCaps := make([]int, len(expaPreCaps))
	for i := range expaCaps {
		expaCaps[i] = max(int(math.Round(float64(min(expaTarget, startTarget))*p.ExpaFinalCapMul)), 1)
	}
	expaMasks, _ := growCompetitiveByCost(
		in.Width,
		in.Height,
		in.ExpaCenters,
		expaCaps,
		blockedExpas,
		func(owner int, idx int, _ int, _ int) bool {
			return in.ExpaOwners[idx] == owner &&
				in.DistWR[idx] > startClearanceTiles &&
				in.DistEdge[idx] > startEdgeClearance &&
				expaOpen[idx] >= p.ExpaMinNeighbors
		},
		func(owner int, idx int, x int, y int) float64 {
			return 1.0 +
				p.ExpaWallW/(1.0+float64(in.DistWR[idx])) +
				p.ExpaEdgeW/(1.0+float64(in.DistEdge[idx])) +
				p.ExpaChokeW*narrowPassagePenalty(expaOpen[idx]) +
				p.ExpaCenterW*distanceToCenterPenalty(in.ExpaCenters[owner], x, y)
		},
	)
	return &buildRegionMasksOutput{
		StartMasks: startMasks,
		ExpaMasks:  expaMasks,
	}, nil
}

func estimateEqualCaps(width int, height int, owners []int, distClear []int, distEdge []int, centers []point, forbidden []bool) []int {
	prelim := growRegionsWithClearance(
		width,
		height,
		owners,
		distClear,
		distEdge,
		centers,
		startClearanceTiles,
		startEdgeClearance,
		forbidden,
	)
	minArea := max(minPositive(regionAreas(prelim)), 1)
	caps := make([]int, len(centers))
	for i := range caps {
		caps[i] = minArea
	}
	return caps
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

func growCompetitiveByCost(
	width int,
	height int,
	centers []point,
	caps []int,
	blocked []bool,
	allow func(owner int, idx int, x int, y int) bool,
	stepCost func(owner int, idx int, x int, y int) float64,
) ([][]bool, []int) {
	const blockedOwner = -2
	ownerGrid := make([]int, width*height)
	for i := range ownerGrid {
		if blocked[i] {
			ownerGrid[i] = blockedOwner
		} else {
			ownerGrid[i] = startOwnerNone
		}
	}
	best := make([][]float64, len(centers))
	for i := range best {
		best[i] = make([]float64, width*height)
		for j := range best[i] {
			best[i][j] = math.MaxFloat64
		}
	}

	areas := make([]int, len(centers))
	pq := growNodeHeap{}
	heap.Init(&pq)
	for i := range centers {
		seedX := clampInt(int(math.Round(centers[i].X)), 0, width-1)
		seedY := clampInt(int(math.Round(centers[i].Y)), 0, height-1)
		x, y, ok := findSeedForOwner(width, height, ownerGrid, i, seedX, seedY, allow)
		if !ok {
			continue
		}
		idx := y*width + x
		best[i][idx] = 0
		heap.Push(&pq, growNode{Cost: 0, Owner: i, Idx: idx})
	}

	for pq.Len() > 0 {
		cur := heap.Pop(&pq).(growNode)
		if cur.Cost > best[cur.Owner][cur.Idx] {
			continue
		}
		if areas[cur.Owner] >= caps[cur.Owner] {
			continue
		}
		if ownerGrid[cur.Idx] == startOwnerNone {
			ownerGrid[cur.Idx] = cur.Owner
			areas[cur.Owner]++
		}
		if ownerGrid[cur.Idx] != cur.Owner {
			continue
		}
		x := cur.Idx % width
		y := cur.Idx / width
		for _, n := range [][2]int{{x + 1, y}, {x - 1, y}, {x, y + 1}, {x, y - 1}} {
			nx, ny := n[0], n[1]
			if nx < 0 || ny < 0 || nx >= width || ny >= height {
				continue
			}
			nidx := ny*width + nx
			if ownerGrid[nidx] == blockedOwner {
				continue
			}
			if ownerGrid[nidx] >= 0 && ownerGrid[nidx] != cur.Owner {
				continue
			}
			if !allow(cur.Owner, nidx, nx, ny) {
				continue
			}
			nextCost := cur.Cost + stepCost(cur.Owner, nidx, nx, ny)
			if nextCost >= best[cur.Owner][nidx] {
				continue
			}
			best[cur.Owner][nidx] = nextCost
			heap.Push(&pq, growNode{Cost: nextCost, Owner: cur.Owner, Idx: nidx})
		}
	}

	regions := make([][]bool, len(centers))
	for i := range regions {
		regions[i] = make([]bool, width*height)
	}
	for idx, o := range ownerGrid {
		if o >= 0 && o < len(regions) {
			regions[o][idx] = true
		}
	}
	return regions, areas
}

func findSeedForOwner(
	width int,
	height int,
	ownerGrid []int,
	owner int,
	x int,
	y int,
	allow func(owner int, idx int, x int, y int) bool,
) (int, int, bool) {
	for r := 0; r <= seedSearchRadius; r++ {
		for yy := y - r; yy <= y+r; yy++ {
			for xx := x - r; xx <= x+r; xx++ {
				if xx < 0 || yy < 0 || xx >= width || yy >= height {
					continue
				}
				idx := yy*width + xx
				if ownerGrid[idx] != startOwnerNone {
					continue
				}
				if !allow(owner, idx, xx, yy) {
					continue
				}
				return xx, yy, true
			}
		}
	}
	return 0, 0, false
}

func walkableNeighborCounts(width int, height int, blocked []bool) []int {
	out := make([]int, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x
			if blocked[idx] {
				continue
			}
			count := 0
			for _, n := range [][2]int{{x + 1, y}, {x - 1, y}, {x, y + 1}, {x, y - 1}} {
				nx, ny := n[0], n[1]
				if nx < 0 || ny < 0 || nx >= width || ny >= height {
					continue
				}
				if !blocked[ny*width+nx] {
					count++
				}
			}
			out[idx] = count
		}
	}
	return out
}

func distanceBarrierPenalty(dist int, scale int) float64 {
	d := max(dist, 0)
	return float64(scale) / (1.0 + float64(d))
}

func narrowPassagePenalty(neighborCount int) float64 {
	switch neighborCount {
	case 0, 1:
		return 3.0
	case 2:
		return 1.4
	case 3:
		return 0.5
	default:
		return 0
	}
}

func distanceToCenterPenalty(center point, x int, y int) float64 {
	dx := center.X - (float64(x) + 0.5)
	dy := center.Y - (float64(y) + 0.5)
	return math.Sqrt(dx*dx + dy*dy)
}
