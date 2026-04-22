package replaymap

import (
	"cmp"
	"math"
	"slices"

	"github.com/marianogappa/scmapanalyzer/internal/model"
)

const (
	centerCircleDiameterFrac = 0.2  // fraction of max(w,h), per spec (diameter 20% of map size)
	centerOverlapMinFrac     = 0.25 // min share of expansion mask cells inside the center circle
)

// nameExpansionBases sets Name and Kind on each expansion polygon after all regions
// and natural links are known. Naturals use kind "natural" and names like
// "natural of 9" or "natural of 3, 6 & 9". At most one non-natural may be named
// "center expa" when uniquely eligible; that polygon's Clock is set to 0 (reserved:
// map center, not a minimap dial hour). Remaining expansions use "expa {clock}"
// with numeric suffixes if needed so all base names are unique.
func nameExpansionBases(meta *model.MapMetadata, w int, h int, expaPolys []BasePolygon, expaMasks [][]bool, startCenters []point, expaCenters []point, naturals []NaturalLink) {
	if len(expaPolys) == 0 {
		return
	}

	startClocks := make([]int, len(startCenters))
	for i := range startCenters {
		startClocks[i] = oclock(meta.WidthTiles, meta.HeightTiles,
			int(math.Round(startCenters[i].X*8)), int(math.Round(startCenters[i].Y*8)))
	}

	startsByExpa := make([][]int, len(expaPolys))
	for _, l := range naturals {
		if l.StartIndex < 0 || l.ExpaIndex < 0 || l.StartIndex >= len(startClocks) || l.ExpaIndex >= len(expaPolys) {
			continue
		}
		startsByExpa[l.ExpaIndex] = append(startsByExpa[l.ExpaIndex], l.StartIndex)
	}
	for i := range startsByExpa {
		startsByExpa[i] = uniqSortedStartIndices(startsByExpa[i])
	}

	naturalIdx := make([]bool, len(expaPolys))
	for ei := range startsByExpa {
		if len(startsByExpa[ei]) > 0 {
			naturalIdx[ei] = true
		}
	}

	centerPick := pickUniqueCenterExpa(w, h, expaMasks, naturalIdx)

	used := make(map[string]bool)

	// 1) Naturals
	for ei := range expaPolys {
		if !naturalIdx[ei] {
			continue
		}
		clocks := make([]int, 0, len(startsByExpa[ei]))
		for _, si := range startsByExpa[ei] {
			clocks = append(clocks, startClocks[si])
		}
		base := formatNaturalOfName(clocks)
		expaPolys[ei].Name = takeUniqueName(used, base)
		expaPolys[ei].Kind = "natural"
	}

	// 2) Center expansion (unique eligible non-natural)
	if centerPick >= 0 {
		expaPolys[centerPick].Name = takeUniqueName(used, "center expa")
		expaPolys[centerPick].Kind = "expa"
		expaPolys[centerPick].Clock = 0
	}

	// 3) Remaining expansions: first pass uses [oclock] (8-way sectors). Bases
	// that would share the same "expa N" name re-use [oclockFloat] and a uniform
	// 1..12 dial ([nearestUniformDialHour]) so 2, 4, 8, 10 can appear. Any
	// remaining string collision gets " (2)" suffixes.
	wt, ht := meta.WidthTiles, meta.HeightTiles
	plainEis := make([]int, 0, len(expaPolys))
	for ei := range expaPolys {
		if expaPolys[ei].Name != "" {
			continue
		}
		plainEis = append(plainEis, ei)
	}

	c1 := make([]int, len(expaPolys))
	hf := make([]float64, len(expaPolys))
	for _, ei := range plainEis {
		if ei < 0 || ei >= len(expaCenters) {
			continue
		}
		px := int(math.Round(expaCenters[ei].X * 8))
		py := int(math.Round(expaCenters[ei].Y * 8))
		c1[ei] = oclock(wt, ht, px, py)
		hf[ei] = oclockFloat(wt, ht, px, py)
	}

	name1 := make(map[int]string, len(plainEis))
	for _, ei := range plainEis {
		name1[ei] = "expa " + itoa(c1[ei])
	}
	conflict := make([]bool, len(expaPolys))
	for _, ei := range plainEis {
		for _, ej := range plainEis {
			if ei >= ej {
				continue
			}
			if name1[ei] == name1[ej] {
				conflict[ei] = true
				conflict[ej] = true
			}
		}
	}

	cFin := make([]int, len(expaPolys))
	for _, ei := range plainEis {
		if conflict[ei] {
			cFin[ei] = nearestUniformDialHour(hf[ei])
		} else {
			cFin[ei] = c1[ei]
		}
	}

	type rem struct {
		ei    int
		clock int
		y, x  int
	}
	remaining := make([]rem, 0, len(plainEis))
	for _, ei := range plainEis {
		ct := expaPolys[ei].CenterTile
		remaining = append(remaining, rem{ei: ei, clock: cFin[ei], y: ct.Y, x: ct.X})
	}
	slices.SortFunc(remaining, func(a, b rem) int {
		if c := cmp.Compare(a.clock, b.clock); c != 0 {
			return c
		}
		if c := cmp.Compare(a.y, b.y); c != 0 {
			return c
		}
		return cmp.Compare(a.x, b.x)
	})
	for _, r := range remaining {
		base := "expa " + itoa(r.clock)
		expaPolys[r.ei].Name = takeUniqueName(used, base)
		expaPolys[r.ei].Kind = "expa"
		expaPolys[r.ei].Clock = r.clock
	}
}

func uniqSortedStartIndices(idxs []int) []int {
	if len(idxs) == 0 {
		return nil
	}
	slices.Sort(idxs)
	out := make([]int, 0, len(idxs))
	prev := -1
	for _, si := range idxs {
		if si == prev {
			continue
		}
		out = append(out, si)
		prev = si
	}
	return out
}

func formatNaturalOfName(clocks []int) string {
	clocks = slices.Clone(clocks)
	slices.Sort(clocks)
	clocks = slices.Compact(clocks)
	if len(clocks) == 0 {
		return "natural"
	}
	if len(clocks) == 1 {
		return "natural of " + itoa(clocks[0])
	}
	if len(clocks) == 2 {
		return "natural of " + itoa(clocks[0]) + " & " + itoa(clocks[1])
	}
	s := "natural of " + itoa(clocks[0])
	for i := 1; i < len(clocks)-1; i++ {
		s += ", " + itoa(clocks[i])
	}
	s += " & " + itoa(clocks[len(clocks)-1])
	return s
}

func takeUniqueName(used map[string]bool, base string) string {
	candidate := base
	for n := 2; used[candidate]; n++ {
		candidate = base + " (" + itoa(n) + ")"
	}
	used[candidate] = true
	return candidate
}

func pickUniqueCenterExpa(w int, h int, expaMasks [][]bool, isNatural []bool) int {
	mapSize := float64(max(w, h))
	radius := 0.5 * centerCircleDiameterFrac * mapSize
	cx := float64(w) / 2
	cy := float64(h) / 2

	eligible := []int{}
	for ei, mask := range expaMasks {
		if ei >= len(isNatural) || isNatural[ei] {
			continue
		}
		frac, tot := maskCenterOverlapFraction(mask, w, h, cx, cy, radius)
		if tot == 0 || frac < centerOverlapMinFrac {
			continue
		}
		eligible = append(eligible, ei)
	}
	if len(eligible) != 1 {
		return -1
	}
	return eligible[0]
}

func maskCenterOverlapFraction(mask []bool, w int, h int, cx float64, cy float64, radius float64) (frac float64, cells int) {
	r2 := radius * radius
	tot := 0
	ins := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := y*w + x
			if !mask[idx] {
				continue
			}
			tot++
			px := float64(x) + 0.5
			py := float64(y) + 0.5
			dx := px - cx
			dy := py - cy
			if dx*dx+dy*dy <= r2 {
				ins++
			}
		}
	}
	if tot == 0 {
		return 0, 0
	}
	return float64(ins) / float64(tot), tot
}

// DuplicateNamesInResult reports base polygon names that appear more than once
// across starting locations and expansions.
func DuplicateNamesInResult(r *Result) []string {
	if r == nil {
		return nil
	}
	count := make(map[string]int)
	for _, p := range r.Starts {
		count[p.Name]++
	}
	for _, p := range r.Expas {
		count[p.Name]++
	}
	out := make([]string, 0)
	for n, c := range count {
		if c > 1 {
			out = append(out, n)
		}
	}
	slices.Sort(out)
	return out
}
