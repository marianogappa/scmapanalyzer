package replaymap

import (
	"container/list"
	"errors"
	"math"
	"sort"

	"github.com/marianogappa/scmapanalyzer/internal/basedetect"
	"github.com/marianogappa/scmapanalyzer/internal/model"
	"github.com/marianogappa/scmapanalyzer/internal/tiletags"
)

const (
	// startOwnerNone marks tiles that are nearest to an expansion base, not a start base.
	// We use -1 because owner indices are otherwise 0..N-1.
	startOwnerNone = -1

	// startClearanceTiles keeps masks at least this many tiles away from
	// wall/ramp barriers when region growing. Increase to make regions tighter.
	startClearanceTiles = 1

	// startEdgeClearance reserves a margin from map edges.
	// Keep at 0 unless edge bleeding appears in generated polygons.
	startEdgeClearance = 0

	// attempt2CapMul caps fallback expansion area as a fraction of the smallest
	// detected start area. Lower values make fallback expansions more conservative.
	attempt2CapMul = 0.88

	// shapeAspectRatio is the major/minor axis ratio used by fixed oblong models.
	// Higher values make bases longer and narrower.
	shapeAspectRatio = 1.7

	// shapeFixedMaxDist is the max normalized ellipse distance allowed during
	// growth. 1.0 hugs the inferred shape; >1.0 allows slight overshoot.
	shapeFixedMaxDist = 1.1

	// shapeMinAxis enforces a minimum semi-axis length for tiny inferred areas.
	shapeMinAxis = 5.0

	// seedSearchRadius is how far from the center we scan for a valid seed tile
	// when the rounded center is blocked by clearance/ownership constraints.
	seedSearchRadius = 10

	// passableSeedSearchRadius is similar to seedSearchRadius but for pathing BFS
	// (natural detection), where we only need a passable tile near a center.
	passableSeedSearchRadius = 12

	// orientationRadius tiles around a base center are sampled to infer the local
	// open-area orientation used to rotate oblong shapes.
	orientationRadius = 16

	// minOrientationSamples avoids unstable covariance orientation on sparse data.
	minOrientationSamples = 10
)

type point struct {
	X float64
	Y float64
}

type shapeModel struct {
	CenterX float64
	CenterY float64
	Angle   float64
	AxisA   float64
	AxisB   float64
}

func Analyze(meta *model.MapMetadata, tags *tiletags.TileSetTags) (*AnalyzeOutput, error) {
	if meta == nil || tags == nil {
		return nil, errors.New("metadata and tile tags are required")
	}
	if len(meta.Tiles) != meta.WidthTiles*meta.HeightTiles {
		return nil, errors.New("tile grid mismatch")
	}

	bases := basedetect.DetectBases(meta.WidthTiles, meta.HeightTiles, meta.MineralFields, meta.Geysers, meta.StartLocations)
	if len(bases) == 0 {
		return nil, errors.New("no bases detected")
	}

	wallSet := asSet(tags.WallTileIDs)
	rampSet := asSet(tags.RampTileIDs)
	wallRampBarrier, edgeBarrier := buildBarriers(meta.WidthTiles, meta.HeightTiles, meta.Tiles, wallSet, rampSet)
	distWR := distanceToBarrier(meta.WidthTiles, meta.HeightTiles, wallRampBarrier)
	distEdge := distanceToBarrier(meta.WidthTiles, meta.HeightTiles, edgeBarrier)

	// Starts: clearance uses wall-only distance so ramp-adjacent tiles inside a wall-bounded
	// natural are not rejected (ramps stay non-walkable via rampForbidden).
	wallOnlyBar := make([]bool, len(meta.Tiles))
	for i, t := range meta.Tiles {
		if wallSet[t] {
			wallOnlyBar[i] = true
		}
	}
	distWallClear := distanceToBarrier(meta.WidthTiles, meta.HeightTiles, wallOnlyBar)

	startBaseIdx, _, startCenters, expaCenters := splitBases(bases)
	if len(startCenters) == 0 || len(expaCenters) == 0 {
		return nil, errors.New("expected both starts and expansions")
	}

	startOwners := assignNearestStartAmongAllBases(meta.WidthTiles, meta.HeightTiles, bases, startBaseIdx)
	rampForbidden := make([]bool, len(meta.Tiles))
	for i, t := range meta.Tiles {
		if rampSet[t] {
			rampForbidden[i] = true
		}
	}
	orientBlockStarts := append([]bool(nil), wallOnlyBar...)
	startMasks, minStartArea := growStartMasksOblong(
		meta.WidthTiles,
		meta.HeightTiles,
		startOwners,
		distWallClear,
		distEdge,
		startCenters,
		startClearanceTiles,
		startEdgeClearance,
		orientBlockStarts,
		rampForbidden,
	)
	if minStartArea <= 0 {
		return nil, errors.New("invalid starting area")
	}

	startOccupied := combineMasks(meta.WidthTiles*meta.HeightTiles, startMasks)
	expaOwners := assignNearest(meta.WidthTiles, meta.HeightTiles, expaCenters)
	expaMasks := make([][]bool, len(expaCenters))
	fallbackIndices := []int{}
	attempt1Blocked := make([]bool, len(startOccupied))
	for i := range attempt1Blocked {
		attempt1Blocked[i] = wallRampBarrier[i] || startOccupied[i]
	}
	attempt1Shapes := inferFixedShapes(meta.WidthTiles, meta.HeightTiles, expaCenters, attempt1Blocked, minStartArea)
	for ei := range expaCenters {
		mask := growSingleRegionWithClearance(
			meta.WidthTiles,
			meta.HeightTiles,
			ei,
			expaOwners,
			distWR,
			distEdge,
			expaCenters[ei],
			startClearanceTiles,
			startEdgeClearance,
			startOccupied,
			&attempt1Shapes[ei],
			shapeFixedMaxDist,
		)
		area := countMask(mask)
		if area > 0 && area <= minStartArea {
			expaMasks[ei] = mask
			continue
		}
		fallbackIndices = append(fallbackIndices, ei)
	}

	if len(fallbackIndices) > 0 {
		attempt2MaxArea := max(int(math.Floor(float64(minStartArea)*attempt2CapMul)), 1)
		a2blocked := make([]bool, len(startOccupied))
		for i := range a2blocked {
			a2blocked[i] = wallRampBarrier[i] || startOccupied[i]
		}
		for ei, mask := range expaMasks {
			if mask == nil || containsInt(fallbackIndices, ei) {
				continue
			}
			for i, v := range mask {
				if v {
					a2blocked[i] = true
				}
			}
		}
		a2Centers := make([]point, len(fallbackIndices))
		for i, expaIdx := range fallbackIndices {
			a2Centers[i] = expaCenters[expaIdx]
		}
		a2Shapes := inferFixedShapes(meta.WidthTiles, meta.HeightTiles, a2Centers, a2blocked, attempt2MaxArea)
		a2Masks := growExpasCompetitiveFixedShape(meta.WidthTiles, meta.HeightTiles, a2Centers, a2blocked, attempt2MaxArea, a2Shapes, shapeFixedMaxDist)
		for i, expaIdx := range fallbackIndices {
			expaMasks[expaIdx] = a2Masks[i]
		}
	}

	naturals := computeNaturals(
		meta.WidthTiles,
		meta.HeightTiles,
		meta.Tiles,
		wallSet,
		startCenters,
		expaCenters,
	)

	startPolys := make([]BasePolygon, len(startCenters))
	for i := range startCenters {
		clock := oclock(meta.WidthTiles, meta.HeightTiles, int(math.Round(startCenters[i].X*32)), int(math.Round(startCenters[i].Y*32)))
		startPolys[i] = BasePolygon{
			Name:            "start " + itoa(clock),
			Kind:            "start",
			Clock:           clock,
			CenterTile:      TilePoint{X: int(math.Round(startCenters[i].X)), Y: int(math.Round(startCenters[i].Y))},
			PolygonVertices: maskToPolygon(meta.WidthTiles, meta.HeightTiles, startMasks[i]),
		}
	}

	expaPolys := make([]BasePolygon, len(expaCenters))
	for i := range expaCenters {
		clock := oclock(meta.WidthTiles, meta.HeightTiles, int(math.Round(expaCenters[i].X*32)), int(math.Round(expaCenters[i].Y*32)))
		expaPolys[i] = BasePolygon{
			Name:            "expa " + itoa(clock),
			Kind:            "expa",
			Clock:           clock,
			CenterTile:      TilePoint{X: int(math.Round(expaCenters[i].X)), Y: int(math.Round(expaCenters[i].Y))},
			PolygonVertices: maskToPolygon(meta.WidthTiles, meta.HeightTiles, expaMasks[i]),
		}
	}

	for _, l := range naturals {
		if l.StartIndex >= 0 && l.StartIndex < len(startPolys) && l.ExpaIndex >= 0 && l.ExpaIndex < len(expaPolys) {
			startPolys[l.StartIndex].NaturalExpansion = expaPolys[l.ExpaIndex].Name
		}
	}

	wallMask := make([]bool, len(meta.Tiles))
	rampMask := make([]bool, len(meta.Tiles))
	for i, t := range meta.Tiles {
		wallMask[i] = wallSet[t]
		rampMask[i] = rampSet[t]
	}

	return &AnalyzeOutput{
		Result: &Result{
			MapName:    meta.MapName,
			TileSetKey: meta.TilesetKey,
			Starts:     startPolys,
			Expas:      expaPolys,
		},
		Debug: &DebugData{
			WidthTiles:   meta.WidthTiles,
			HeightTiles:  meta.HeightTiles,
			StartMasks:   startMasks,
			ExpaMasks:    expaMasks,
			NaturalLinks: naturals,
			WallMask:     wallMask,
			RampMask:     rampMask,
		},
		Bases: bases,
	}, nil
}

func splitBases(bases []model.Base) ([]int, []int, []point, []point) {
	startIdx := []int{}
	expaIdx := []int{}
	startCenters := []point{}
	expaCenters := []point{}
	for i, b := range bases {
		c := point{X: b.CenterX / 32.0, Y: b.CenterY / 32.0}
		if b.IsStarting {
			startIdx = append(startIdx, i)
			startCenters = append(startCenters, c)
			continue
		}
		expaIdx = append(expaIdx, i)
		expaCenters = append(expaCenters, c)
	}
	return startIdx, expaIdx, startCenters, expaCenters
}

func asSet(ids []uint16) map[uint16]bool {
	out := map[uint16]bool{}
	for _, id := range ids {
		out[id] = true
	}
	return out
}

func containsInt(values []int, target int) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}

func combineMasks(size int, masks [][]bool) []bool {
	out := make([]bool, size)
	for _, m := range masks {
		for i, v := range m {
			if v {
				out[i] = true
			}
		}
	}
	return out
}

func buildBarriers(width int, height int, tiles []uint16, wallSet map[uint16]bool, rampSet map[uint16]bool) ([]bool, []bool) {
	wr := make([]bool, width*height)
	edge := make([]bool, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x
			if x == 0 || y == 0 || x == width-1 || y == height-1 {
				edge[idx] = true
			}
			if wallSet[tiles[idx]] || rampSet[tiles[idx]] {
				wr[idx] = true
			}
		}
	}
	return wr, edge
}

func distanceToBarrier(width int, height int, barrier []bool) []int {
	const inf = int(^uint(0) >> 1)
	dist := make([]int, width*height)
	q := list.New()
	for i := range dist {
		if barrier[i] {
			dist[i] = 0
			q.PushBack(i)
			continue
		}
		dist[i] = inf
	}
	for q.Len() > 0 {
		e := q.Front()
		q.Remove(e)
		idx := e.Value.(int)
		x := idx % width
		y := idx / width
		nextD := dist[idx] + 1
		for _, n := range [][2]int{{x + 1, y}, {x - 1, y}, {x, y + 1}, {x, y - 1}} {
			nx, ny := n[0], n[1]
			if nx < 0 || ny < 0 || nx >= width || ny >= height {
				continue
			}
			nidx := ny*width + nx
			if nextD < dist[nidx] {
				dist[nidx] = nextD
				q.PushBack(nidx)
			}
		}
	}
	return dist
}

func assignNearest(width int, height int, centers []point) []int {
	owners := make([]int, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			px := float64(x) + 0.5
			py := float64(y) + 0.5
			best := 0
			bestD := math.MaxFloat64
			for i, c := range centers {
				dx := c.X - px
				dy := c.Y - py
				d := dx*dx + dy*dy
				if d < bestD {
					bestD = d
					best = i
				}
			}
			owners[y*width+x] = best
		}
	}
	return owners
}

// assignNearestStartAmongAllBases assigns each tile to the nearest detected base (start or expansion).
// Tiles whose nearest base is an expansion get startOwnerNone so starting regions cannot grow over them.
func assignNearestStartAmongAllBases(width int, height int, bases []model.Base, startOrderedBaseIdx []int) []int {
	centers := make([]point, len(bases))
	for i, b := range bases {
		centers[i] = point{X: b.CenterX / 32.0, Y: b.CenterY / 32.0}
	}
	baseToStartOrd := make([]int, len(bases))
	for i := range baseToStartOrd {
		baseToStartOrd[i] = startOwnerNone
	}
	for si, bi := range startOrderedBaseIdx {
		baseToStartOrd[bi] = si
	}
	owners := make([]int, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			px := float64(x) + 0.5
			py := float64(y) + 0.5
			best := -1
			bestD := math.MaxFloat64
			for i, c := range centers {
				dx := c.X - px
				dy := c.Y - py
				d := dx*dx + dy*dy
				if d < bestD {
					bestD = d
					best = i
				}
			}
			if best < 0 {
				owners[y*width+x] = startOwnerNone
				continue
			}
			owners[y*width+x] = baseToStartOrd[best]
		}
	}
	return owners
}

func growRegionsWithClearance(width int, height int, owners []int, distClear []int, distEdge []int, centers []point, clearance int, edgeClearance int, forbidden []bool) [][]bool {
	regions := make([][]bool, len(centers))
	for i := range centers {
		regions[i] = growSingleRegionWithClearance(width, height, i, owners, distClear, distEdge, centers[i], clearance, edgeClearance, forbidden, nil, 0)
	}
	return regions
}

// growStartMasksOblong first grows unconstrained Voronoi start regions to find the smallest
// footprint among players (same budget idea as expansion attempt 1), then regrows each
// start as the intersection of that Voronoi cell with a centroid-based oblong
// (inferFixedShapes + shapeDistance), matching expansion geometry.
func growStartMasksOblong(width int, height int, owners []int, distClear []int, distEdge []int, centers []point, clearance int, edgeClearance int, orientBlocked []bool, forbidden []bool) ([][]bool, int) {
	prelim := growRegionsWithClearance(width, height, owners, distClear, distEdge, centers, clearance, edgeClearance, forbidden)
	minPre := minPositive(regionAreas(prelim))
	if minPre <= 0 {
		return nil, 0
	}
	shapes := inferFixedShapes(width, height, centers, orientBlocked, minPre)
	out := make([][]bool, len(centers))
	for i := range centers {
		out[i] = growSingleRegionWithClearance(
			width,
			height,
			i,
			owners,
			distClear,
			distEdge,
			centers[i],
			clearance,
			edgeClearance,
			forbidden,
			&shapes[i],
			shapeFixedMaxDist,
		)
	}
	return out, minPositive(regionAreas(out))
}

func growSingleRegionWithClearance(width int, height int, owner int, owners []int, distClear []int, distEdge []int, center point, clearance int, edgeClearance int, forbidden []bool, shape *shapeModel, maxShapeDist float64) []bool {
	mask := make([]bool, width*height)
	seedX := clampInt(int(math.Round(center.X)), 0, width-1)
	seedY := clampInt(int(math.Round(center.Y)), 0, height-1)
	seedX, seedY, ok := findSeed(width, height, owners, distClear, distEdge, owner, seedX, seedY, clearance, edgeClearance, forbidden)
	if !ok {
		return mask
	}
	queue := []int{seedY*width + seedX}
	mask[seedY*width+seedX] = true
	for q := 0; q < len(queue); q++ {
		idx := queue[q]
		x := idx % width
		y := idx / width
		for _, n := range [][2]int{{x + 1, y}, {x - 1, y}, {x, y + 1}, {x, y - 1}} {
			nx, ny := n[0], n[1]
			if nx < 0 || ny < 0 || nx >= width || ny >= height {
				continue
			}
			nidx := ny*width + nx
			if mask[nidx] || owners[nidx] != owner {
				continue
			}
			if distClear[nidx] <= clearance || distEdge[nidx] <= edgeClearance {
				continue
			}
			if forbidden != nil && forbidden[nidx] {
				continue
			}
			if shape != nil && shapeDistance(center, *shape, nx, ny) > maxShapeDist {
				continue
			}
			mask[nidx] = true
			queue = append(queue, nidx)
		}
	}
	return mask
}

func findSeed(width int, height int, owners []int, distClear []int, distEdge []int, owner int, x int, y int, clearance int, edgeClearance int, forbidden []bool) (int, int, bool) {
	for r := 0; r <= seedSearchRadius; r++ {
		for yy := y - r; yy <= y+r; yy++ {
			for xx := x - r; xx <= x+r; xx++ {
				if xx < 0 || yy < 0 || xx >= width || yy >= height {
					continue
				}
				idx := yy*width + xx
				if owners[idx] != owner {
					continue
				}
				if distClear[idx] <= clearance || distEdge[idx] <= edgeClearance {
					continue
				}
				if forbidden != nil && forbidden[idx] {
					continue
				}
				return xx, yy, true
			}
		}
	}
	return 0, 0, false
}

func inferFixedShapes(width int, height int, centers []point, blocked []bool, maxArea int) []shapeModel {
	shapes := make([]shapeModel, len(centers))
	for i, c := range centers {
		angle := localOrientation(width, height, c, blocked, orientationRadius)
		axisB := math.Sqrt(float64(maxArea) / (math.Pi * shapeAspectRatio))
		axisA := axisB * shapeAspectRatio
		if axisA < shapeMinAxis {
			axisA = shapeMinAxis
		}
		if axisB < shapeMinAxis {
			axisB = shapeMinAxis
		}
		shapes[i] = shapeModel{CenterX: c.X, CenterY: c.Y, Angle: angle, AxisA: axisA, AxisB: axisB}
	}
	return shapes
}

func localOrientation(width int, height int, c point, blocked []bool, radius int) float64 {
	cx := clampInt(int(math.Round(c.X)), 0, width-1)
	cy := clampInt(int(math.Round(c.Y)), 0, height-1)
	varXX := 0.0
	varYY := 0.0
	varXY := 0.0
	n := 0.0
	for y := cy - radius; y <= cy+radius; y++ {
		if y < 0 || y >= height {
			continue
		}
		for x := cx - radius; x <= cx+radius; x++ {
			if x < 0 || x >= width {
				continue
			}
			idx := y*width + x
			if blocked[idx] {
				continue
			}
			dx := float64(x-cx) + 0.5
			dy := float64(y-cy) + 0.5
			if dx*dx+dy*dy > float64(radius*radius) {
				continue
			}
			n++
			varXX += dx * dx
			varYY += dy * dy
			varXY += dx * dy
		}
	}
	if n < minOrientationSamples {
		return 0
	}
	varXX /= n
	varYY /= n
	varXY /= n
	return 0.5 * math.Atan2(2*varXY, varXX-varYY)
}

func shapeDistance(center point, shape shapeModel, x int, y int) float64 {
	px := float64(x) + 0.5
	py := float64(y) + 0.5
	cx := shape.CenterX
	cy := shape.CenterY
	if cx == 0 && cy == 0 {
		cx = center.X
		cy = center.Y
	}
	a := shape.AxisA
	b := shape.AxisB
	if a <= 0 || b <= 0 {
		a = shapeMinAxis * shapeAspectRatio
		b = shapeMinAxis
	}
	dx := px - cx
	dy := py - cy
	cosT := math.Cos(shape.Angle)
	sinT := math.Sin(shape.Angle)
	u := dx*cosT + dy*sinT
	v := -dx*sinT + dy*cosT
	return math.Sqrt((u*u)/(a*a) + (v*v)/(b*b))
}

func growExpasCompetitiveFixedShape(width int, height int, centers []point, blocked []bool, maxArea int, shapes []shapeModel, maxShapeDist float64) [][]bool {
	return growExpasCompetitiveWithGate(width, height, centers, blocked, maxArea, func(i int, x int, y int, _ []bool) bool {
		return shapeDistance(centers[i], shapes[i], x, y) <= maxShapeDist
	})
}

func growExpasCompetitiveWithGate(width int, height int, centers []point, blocked []bool, maxArea int, allow func(i int, x int, y int, region []bool) bool) [][]bool {
	const unclaimed = -1
	owner := make([]int, width*height)
	for i := range owner {
		if blocked[i] {
			owner[i] = -2
		} else {
			owner[i] = unclaimed
		}
	}
	regions := make([][]bool, len(centers))
	areas := make([]int, len(centers))
	frontiers := make([][]int, len(centers))
	for i := range centers {
		regions[i] = make([]bool, width*height)
		x := clampInt(int(math.Round(centers[i].X)), 0, width-1)
		y := clampInt(int(math.Round(centers[i].Y)), 0, height-1)
		x, y, ok := findUnblocked(width, height, owner, x, y)
		if !ok {
			continue
		}
		idx := y*width + x
		owner[idx] = i
		regions[i][idx] = true
		areas[i] = 1
		frontiers[i] = []int{idx}
	}
	for {
		proposals := map[int][]int{}
		for i := range centers {
			if areas[i] >= maxArea || len(frontiers[i]) == 0 {
				continue
			}
			for _, idx := range frontiers[i] {
				x := idx % width
				y := idx / width
				for _, n := range [][2]int{{x + 1, y}, {x - 1, y}, {x, y + 1}, {x, y - 1}} {
					nx, ny := n[0], n[1]
					if nx < 0 || ny < 0 || nx >= width || ny >= height {
						continue
					}
					nidx := ny*width + nx
					if owner[nidx] != unclaimed {
						continue
					}
					if !allow(i, nx, ny, regions[i]) {
						continue
					}
					proposals[nidx] = appendUnique(proposals[nidx], i)
				}
			}
		}
		if len(proposals) == 0 {
			break
		}
		nextFrontiers := make([][]int, len(centers))
		progress := false
		for nidx, who := range proposals {
			if len(who) == 1 {
				i := who[0]
				if areas[i] >= maxArea || owner[nidx] != unclaimed {
					continue
				}
				owner[nidx] = i
				regions[i][nidx] = true
				areas[i]++
				nextFrontiers[i] = append(nextFrontiers[i], nidx)
				progress = true
			} else {
				owner[nidx] = -2
			}
		}
		for i := range centers {
			frontiers[i] = nextFrontiers[i]
		}
		if !progress {
			break
		}
	}
	return regions
}

func findUnblocked(width int, height int, owner []int, x int, y int) (int, int, bool) {
	for r := 0; r <= seedSearchRadius; r++ {
		for yy := y - r; yy <= y+r; yy++ {
			for xx := x - r; xx <= x+r; xx++ {
				if xx < 0 || yy < 0 || xx >= width || yy >= height {
					continue
				}
				if owner[yy*width+xx] == -1 {
					return xx, yy, true
				}
			}
		}
	}
	return 0, 0, false
}

func computeNaturals(width int, height int, tiles []uint16, wallSet map[uint16]bool, starts []point, expas []point) []NaturalLink {
	blocked := make([]bool, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x
			if x == 0 || y == 0 || x == width-1 || y == height-1 {
				blocked[idx] = true
				continue
			}
			if wallSet[tiles[idx]] {
				blocked[idx] = true
			}
		}
	}
	links := make([]NaturalLink, 0, len(starts))
	for si, s := range starts {
		sx, sy, ok := nearestPassableSeed(width, height, blocked, int(math.Round(s.X)), int(math.Round(s.Y)))
		if !ok {
			continue
		}
		dist, prev := bfsFrom(width, height, blocked, sx, sy)
		bestExpa := -1
		bestDist := int(^uint(0) >> 1)
		bestPath := []TilePoint{}
		for ei, e := range expas {
			ex, ey, ok := nearestPassableSeed(width, height, blocked, int(math.Round(e.X)), int(math.Round(e.Y)))
			if !ok {
				continue
			}
			d := dist[ey*width+ex]
			if d < 0 || d >= bestDist {
				continue
			}
			path := reconstructPath(width, prev, sx, sy, ex, ey)
			if len(path) == 0 {
				continue
			}
			bestDist = d
			bestExpa = ei
			bestPath = path
		}
		if bestExpa >= 0 {
			links = append(links, NaturalLink{StartIndex: si, ExpaIndex: bestExpa, Path: bestPath})
		}
	}
	return links
}

func bfsFrom(width int, height int, blocked []bool, sx int, sy int) ([]int, []int) {
	dist := make([]int, width*height)
	prev := make([]int, width*height)
	for i := range dist {
		dist[i] = -1
		prev[i] = -1
	}
	startIdx := sy*width + sx
	dist[startIdx] = 0
	q := list.New()
	q.PushBack(startIdx)
	for q.Len() > 0 {
		e := q.Front()
		q.Remove(e)
		idx := e.Value.(int)
		x := idx % width
		y := idx / width
		for _, n := range [][2]int{{x + 1, y}, {x - 1, y}, {x, y + 1}, {x, y - 1}} {
			nx, ny := n[0], n[1]
			if nx < 0 || ny < 0 || nx >= width || ny >= height {
				continue
			}
			nidx := ny*width + nx
			if blocked[nidx] || dist[nidx] >= 0 {
				continue
			}
			dist[nidx] = dist[idx] + 1
			prev[nidx] = idx
			q.PushBack(nidx)
		}
	}
	return dist, prev
}

func nearestPassableSeed(width int, height int, blocked []bool, x int, y int) (int, int, bool) {
	x = clampInt(x, 0, width-1)
	y = clampInt(y, 0, height-1)
	for r := 0; r <= passableSeedSearchRadius; r++ {
		for yy := y - r; yy <= y+r; yy++ {
			for xx := x - r; xx <= x+r; xx++ {
				if xx < 0 || yy < 0 || xx >= width || yy >= height {
					continue
				}
				if !blocked[yy*width+xx] {
					return xx, yy, true
				}
			}
		}
	}
	return 0, 0, false
}

func reconstructPath(width int, prev []int, sx int, sy int, tx int, ty int) []TilePoint {
	startIdx := sy*width + sx
	targetIdx := ty*width + tx
	if targetIdx != startIdx && prev[targetIdx] < 0 {
		return nil
	}
	reverse := []TilePoint{}
	for cur := targetIdx; cur >= 0; cur = prev[cur] {
		reverse = append(reverse, TilePoint{X: cur % width, Y: cur / width})
		if cur == startIdx {
			break
		}
	}
	if len(reverse) == 0 || reverse[len(reverse)-1].X != sx || reverse[len(reverse)-1].Y != sy {
		return nil
	}
	out := make([]TilePoint, len(reverse))
	for i := range reverse {
		out[i] = reverse[len(reverse)-1-i]
	}
	return out
}

func maskToPolygon(width int, height int, mask []bool) []TilePoint {
	if len(mask) == 0 {
		return nil
	}
	boundary := []TilePoint{}
	cx := 0.0
	cy := 0.0
	count := 0.0
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x
			if !mask[idx] {
				continue
			}
			count++
			cx += float64(x)
			cy += float64(y)
			isBoundary := x == 0 || y == 0 || x == width-1 || y == height-1
			if !isBoundary {
				if !mask[idx-1] || !mask[idx+1] || !mask[idx-width] || !mask[idx+width] {
					isBoundary = true
				}
			}
			if isBoundary {
				boundary = append(boundary, TilePoint{X: x, Y: y})
			}
		}
	}
	if len(boundary) == 0 {
		return nil
	}
	if count > 0 {
		cx /= count
		cy /= count
	}
	sort.Slice(boundary, func(i int, j int) bool {
		ai := math.Atan2(float64(boundary[i].Y)-cy, float64(boundary[i].X)-cx)
		aj := math.Atan2(float64(boundary[j].Y)-cy, float64(boundary[j].X)-cx)
		return ai < aj
	})
	return boundary
}

func oclock(widthTiles int, heightTiles int, x int, y int) int {
	mapWidth := widthTiles * 32
	mapHeight := heightTiles * 32
	centerX := float64(mapWidth) / 2
	centerY := float64(mapHeight) / 2
	angle := math.Atan2(float64(y)-centerY, float64(x)-centerX) * 180.0 / math.Pi
	if angle < 0 {
		angle += 360
	}
	switch {
	case angle >= 337.5 || angle < 22.5:
		return 3
	case angle >= 22.5 && angle < 67.5:
		return 5
	case angle >= 67.5 && angle < 112.5:
		return 6
	case angle >= 112.5 && angle < 157.5:
		return 7
	case angle >= 157.5 && angle < 202.5:
		return 9
	case angle >= 202.5 && angle < 247.5:
		return 11
	case angle >= 247.5 && angle < 292.5:
		return 12
	default:
		return 1
	}
}

func appendUnique(values []int, v int) []int {
	for _, x := range values {
		if x == v {
			return values
		}
	}
	return append(values, v)
}

func regionAreas(regions [][]bool) []int {
	out := make([]int, len(regions))
	for i, m := range regions {
		out[i] = countMask(m)
	}
	return out
}

func countMask(mask []bool) int {
	c := 0
	for _, v := range mask {
		if v {
			c++
		}
	}
	return c
}

func minPositive(values []int) int {
	best := 0
	for _, v := range values {
		if v <= 0 {
			continue
		}
		if best == 0 || v < best {
			best = v
		}
	}
	return best
}

func clampInt(v int, lo int, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := false
	if v < 0 {
		neg = true
		v = -v
	}
	digits := []byte{}
	for v > 0 {
		digits = append(digits, byte('0'+(v%10)))
		v /= 10
	}
	if neg {
		digits = append(digits, '-')
	}
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return string(digits)
}
