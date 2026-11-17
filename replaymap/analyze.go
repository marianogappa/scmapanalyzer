package replaymap

import (
	"container/list"
	"errors"
	"math"

	"github.com/marianogappa/scmapanalyzer/internal/basedetect"
	"github.com/marianogappa/scmapanalyzer/internal/mapgfx"
	"github.com/marianogappa/scmapanalyzer/internal/model"
)

const (
	// passableSeedSearchRadius is how far from a base center we scan for a
	// passable tile to start pathing or region growth from.
	passableSeedSearchRadius = 12

	// geyserReachPx is how far (pixels) a geyser may sit from a base's resource
	// center and still be that base's gas. Across the regression corpus real
	// gas sits at <=391px and the nearest foreign geyser at >=416px.
	geyserReachPx = 400
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

	startBaseIdx, expaBaseIdx, startCenters, expaCenters := splitBases(bases)
	if len(startCenters) == 0 || len(expaCenters) == 0 {
		return nil, errors.New("expected both starts and expansions")
	}

	terrain := computeTerrainAreas(w, h, grid.Data, grid.Elevation)
	seeds := make([]baseSeed, 0, len(startCenters)+len(expaCenters))
	for _, c := range startCenters {
		seeds = append(seeds, baseSeed{Center: c, Start: true})
	}
	for _, c := range expaCenters {
		seeds = append(seeds, baseSeed{Center: c})
	}
	regions := buildBaseRegions(terrain, seeds)

	naturalBlocked := naturalBlockedMask(w, h, grid.Data)
	naturals := computeNaturals(w, h, naturalBlocked, startCenters, expaCenters)

	startPolys := make([]BasePolygon, len(startCenters))
	for i := range startCenters {
		clock := oclock(meta.WidthTiles, meta.HeightTiles, int(math.Round(startCenters[i].X*8)), int(math.Round(startCenters[i].Y*8)))
		startPolys[i] = BasePolygon{
			Name:            "start " + itoa(clock),
			Kind:            "start",
			Clock:           clock,
			CenterTile:      TilePoint{X: int(math.Round(startCenters[i].X)), Y: int(math.Round(startCenters[i].Y))},
			PolygonVertices: maskOutline(w, h, regions.Masks[i]),
			MineralOnly:     mineralOnly(bases, startBaseIdx[i], meta.Geysers),
		}
	}

	expaPolys := make([]BasePolygon, len(expaCenters))
	for i := range expaCenters {
		clock := oclock(meta.WidthTiles, meta.HeightTiles, int(math.Round(expaCenters[i].X*8)), int(math.Round(expaCenters[i].Y*8)))
		expaPolys[i] = BasePolygon{
			Clock:           clock,
			CenterTile:      TilePoint{X: int(math.Round(expaCenters[i].X)), Y: int(math.Round(expaCenters[i].Y))},
			PolygonVertices: maskOutline(w, h, regions.Masks[len(startCenters)+i]),
			MineralOnly:     mineralOnly(bases, expaBaseIdx[i], meta.Geysers),
		}
	}
	nameExpansionBases(meta, w, h, expaPolys, regions.Masks[len(startCenters):], startCenters, expaCenters, naturals)

	attachChokes(regions, startPolys, expaPolys)

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
			StartMasks:      regions.Masks[:len(startCenters)],
			ExpaMasks:       regions.Masks[len(startCenters):],
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

func oclock(widthTiles int, heightTiles int, x int, y int) int {
	angle := rayAngleDegrees(widthTiles, heightTiles, x, y)
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

// mineralOnly reports whether no geyser belongs to the base: a geyser belongs
// to the base whose resource cluster center is nearest, if within reach.
func mineralOnly(bases []model.Base, base int, geysers []model.MapResource) bool {
	for _, g := range geysers {
		gx, gy := float64(g.X), float64(g.Y)
		nearest, best := -1, math.MaxFloat64
		for i, b := range bases {
			if d := math.Hypot(b.CenterX-gx, b.CenterY-gy); d < best {
				nearest, best = i, d
			}
		}
		if nearest == base && best <= geyserReachPx {
			return false
		}
	}
	return true
}

func attachChokes(regions *baseRegions, starts, expas []BasePolygon) {
	at := func(i int) *BasePolygon {
		if i < len(starts) {
			return &starts[i]
		}
		return &expas[i-len(starts)]
	}
	for i, segs := range regions.Chokes {
		b := at(i)
		for _, s := range segs {
			c := Choke{Center: s.Center, Ends: [2]TilePoint{s.A, s.B}, Width: int(math.Round(s.Width))}
			if s.OtherBase >= 0 {
				c.ConnectsTo = at(s.OtherBase).Name
			}
			b.Chokes = append(b.Chokes, c)
		}
	}
}
