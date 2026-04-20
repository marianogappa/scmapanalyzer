package replaymap

import (
	"container/list"
	"errors"
	"math"
	"sort"

	"github.com/marianogappa/scmapanalyzer/internal/basedetect"
	"github.com/marianogappa/scmapanalyzer/internal/mapgfx"
	"github.com/marianogappa/scmapanalyzer/internal/model"
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

	// seedSearchRadius is how far from the center we scan for a valid seed tile
	// when the rounded center is blocked by clearance/ownership constraints.
	seedSearchRadius = 10

	// passableSeedSearchRadius is similar to seedSearchRadius but for pathing BFS
	// (natural detection), where we only need a passable tile near a center.
	passableSeedSearchRadius = 12
)

type point struct {
	X float64
	Y float64
}

func Analyze(meta *model.MapMetadata) (*AnalyzeOutput, error) {
	if meta == nil {
		return nil, errors.New("metadata is required")
	}
	if len(meta.Tiles) != meta.WidthTiles*meta.HeightTiles {
		return nil, errors.New("tile grid mismatch")
	}

	folder, err := mapgfx.TilesetAssetFolderFromReplay(meta.TilesetKey)
	if err != nil {
		return nil, err
	}
	grid, err := mapgfx.BuildMiniTileGrid(folder, meta.WidthTiles, meta.HeightTiles, meta.Tiles)
	if err != nil {
		return nil, err
	}
	w := grid.Width
	h := grid.Height

	bases := basedetect.DetectBases(meta.WidthTiles, meta.HeightTiles, meta.MineralFields, meta.Geysers, meta.StartLocations)
	if len(bases) == 0 {
		return nil, errors.New("no bases detected")
	}

	wallRampBarrier, edgeBarrier, wallOnlyBar, rampForbidden := barriersFromMiniTileGrid(w, h, grid.Data)
	distWR := distanceToBarrier(w, h, wallRampBarrier)
	distEdge := distanceToBarrier(w, h, edgeBarrier)
	distWallClear := distanceToBarrier(w, h, wallOnlyBar)

	startBaseIdx, _, startCenters, expaCenters := splitBases(bases)
	if len(startCenters) == 0 || len(expaCenters) == 0 {
		return nil, errors.New("expected both starts and expansions")
	}

	startOwners := assignNearestStartAmongAllBases(w, h, bases, startBaseIdx)
	expaOwners := assignNearest(w, h, expaCenters)
	maskOut, maskErr := buildRegionMasks(buildRegionMasksInput{
		Width:           w,
		Height:          h,
		DistWR:          distWR,
		DistEdge:        distEdge,
		DistWallOnly:    distWallClear,
		WallRampBarrier: wallRampBarrier,
		WallOnlyBarrier: wallOnlyBar,
		StartOwners:     startOwners,
		ExpaOwners:      expaOwners,
		StartCenters:    startCenters,
		ExpaCenters:     expaCenters,
		RampForbidden:   rampForbidden,
	})
	if maskErr != nil {
		return nil, maskErr
	}
	startMasks := maskOut.StartMasks
	expaMasks := maskOut.ExpaMasks

	naturalBlocked := naturalBlockedMask(w, h, grid.Data)
	naturals := computeNaturals(w, h, naturalBlocked, startCenters, expaCenters)

	startPolys := make([]BasePolygon, len(startCenters))
	for i := range startCenters {
		clock := oclock(meta.WidthTiles, meta.HeightTiles, int(math.Round(startCenters[i].X*8)), int(math.Round(startCenters[i].Y*8)))
		startMask := startMasks[i]
		startPolys[i] = BasePolygon{
			Name:            "start " + itoa(clock),
			Kind:            "start",
			Clock:           clock,
			CenterTile:      TilePoint{X: int(math.Round(startCenters[i].X)), Y: int(math.Round(startCenters[i].Y))},
			PolygonVertices: maskToPolygon(w, h, startMask),
			MineralOnly:     mineralOnlyMask(w, h, startMask, meta.Geysers),
		}
	}

	expaPolys := make([]BasePolygon, len(expaCenters))
	for i := range expaCenters {
		clock := oclock(meta.WidthTiles, meta.HeightTiles, int(math.Round(expaCenters[i].X*8)), int(math.Round(expaCenters[i].Y*8)))
		expaMask := expaMasks[i]
		expaPolys[i] = BasePolygon{
			Name:            "expa " + itoa(clock),
			Kind:            "expa",
			Clock:           clock,
			CenterTile:      TilePoint{X: int(math.Round(expaCenters[i].X)), Y: int(math.Round(expaCenters[i].Y))},
			PolygonVertices: maskToPolygon(w, h, expaMask),
			MineralOnly:     mineralOnlyMask(w, h, expaMask, meta.Geysers),
		}
	}

	for _, l := range naturals {
		if l.StartIndex >= 0 && l.StartIndex < len(startPolys) && l.ExpaIndex >= 0 && l.ExpaIndex < len(expaPolys) {
			startPolys[l.StartIndex].NaturalExpansion = expaPolys[l.ExpaIndex].Name
		}
	}

	wallMask := make([]bool, len(grid.Data))
	rampMask := make([]bool, len(grid.Data))
	for i, feat := range grid.Data {
		f := feat & 0x0F
		walk := (f & mapgfx.FeatureWalkable) != 0
		ramp := (f & mapgfx.FeatureRamp) != 0
		wallMask[i] = !walk && !ramp
		rampMask[i] = ramp
	}

	return &AnalyzeOutput{
		Result: &Result{
			MapName:    meta.MapName,
			TileSetKey: meta.TilesetKey,
			Starts:     startPolys,
			Expas:      expaPolys,
		},
		Debug: &DebugData{
			WidthMinitiles:  w,
			HeightMinitiles: h,
			StartMasks:      startMasks,
			ExpaMasks:       expaMasks,
			NaturalLinks:    naturals,
			WallMask:        wallMask,
			RampMask:        rampMask,
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
		c := point{X: b.CenterX / 8.0, Y: b.CenterY / 8.0}
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

func barriersFromMiniTileGrid(width, height int, data []uint8) (wallRamp, edge, wallOnly, rampForbidden []bool) {
	n := width * height
	wallRamp = make([]bool, n)
	edge = make([]bool, n)
	wallOnly = make([]bool, n)
	rampForbidden = make([]bool, n)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x
			if x == 0 || y == 0 || x == width-1 || y == height-1 {
				edge[idx] = true
			}
			f := data[idx] & 0x0F
			walk := (f & mapgfx.FeatureWalkable) != 0
			ramp := (f & mapgfx.FeatureRamp) != 0
			if !walk || ramp {
				wallRamp[idx] = true
			}
			if !walk && !ramp {
				wallOnly[idx] = true
			}
			if ramp {
				rampForbidden[idx] = true
			}
		}
	}
	return wallRamp, edge, wallOnly, rampForbidden
}

func naturalBlockedMask(width, height int, data []uint8) []bool {
	out := make([]bool, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x
			if x == 0 || y == 0 || x == width-1 || y == height-1 {
				out[idx] = true
				continue
			}
			f := data[idx] & 0x0F
			walk := (f & mapgfx.FeatureWalkable) != 0
			ramp := (f & mapgfx.FeatureRamp) != 0
			out[idx] = !walk && !ramp
		}
	}
	return out
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
		centers[i] = point{X: b.CenterX / 8.0, Y: b.CenterY / 8.0}
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
		regions[i] = growSingleRegionWithClearance(width, height, i, owners, distClear, distEdge, centers[i], clearance, edgeClearance, forbidden)
	}
	return regions
}

func growSingleRegionWithClearance(width int, height int, owner int, owners []int, distClear []int, distEdge []int, center point, clearance int, edgeClearance int, forbidden []bool) []bool {
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

func computeNaturals(width int, height int, blocked []bool, starts []point, expas []point) []NaturalLink {
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

func mineralOnlyMask(width int, height int, mask []bool, geysers []model.MapResource) bool {
	for _, g := range geysers {
		gx := clampInt(int(math.Round(float64(g.X)/8.0)), 0, width-1)
		gy := clampInt(int(math.Round(float64(g.Y)/8.0)), 0, height-1)
		for oy := -1; oy <= 1; oy++ {
			for ox := -1; ox <= 1; ox++ {
				tx := clampInt(gx+ox, 0, width-1)
				ty := clampInt(gy+oy, 0, height-1)
				if mask[ty*width+tx] {
					return false
				}
			}
		}
	}
	return true
}
