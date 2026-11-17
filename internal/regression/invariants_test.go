package regression

import "testing"

// invariant is the semantic spec for one map: the things that are simply right
// or wrong about a detection, independent of exact polygon geometry. Unlike the
// golden files, this table is NEVER regenerated automatically — you edit it by
// hand, on purpose, in review. It is the contract the algorithm must keep while
// you refactor or simplify it.
type invariant struct {
	starts               int
	expas                int
	centerBases          int // bases inside the map's center circle (see geometry.go)
	everyStartHasNatural bool
	chokedStarts         int // starts with a choke leading straight into their natural
}

// expected captures CURRENT accepted behavior across the corpus. Keep this green.
// When you intentionally change detection output, bump the affected entry in the
// same PR so the diff records the decision.
var expected = map[string]invariant{
	"1-3":                     {starts: 4, expas: 9, centerBases: 1, everyStartHasNatural: true, chokedStarts: 4},
	"1-4":                     {starts: 4, expas: 9, centerBases: 1, everyStartHasNatural: true, chokedStarts: 4},
	"a-iolos-1-0b":            {starts: 3, expas: 12, centerBases: 0, everyStartHasNatural: true, chokedStarts: 0},
	"a-ttitude-1-0":           {starts: 4, expas: 12, centerBases: 0, everyStartHasNatural: true, chokedStarts: 4},
	"big-game-hunters":        {starts: 8, expas: 9, centerBases: 1, everyStartHasNatural: true, chokedStarts: 7},
	"c-olorless-f-ate-1-1":    {starts: 2, expas: 10, centerBases: 0, everyStartHasNatural: true, chokedStarts: 2},
	"d-o-m-i-n-a-t-o-r-se-2":  {starts: 3, expas: 12, centerBases: 0, everyStartHasNatural: true, chokedStarts: 3},
	"j-ane-d-oe-1-2":          {starts: 2, expas: 10, centerBases: 0, everyStartHasNatural: true, chokedStarts: 2},
	"knockout-1-4":            {starts: 4, expas: 12, centerBases: 0, everyStartHasNatural: true, chokedStarts: 4},
	"l-a-c-ampanella-1-1":     {starts: 4, expas: 13, centerBases: 1, everyStartHasNatural: true, chokedStarts: 1},
	"la-mancha-1-1":           {starts: 5, expas: 7, centerBases: 0, everyStartHasNatural: true, chokedStarts: 4},
	"lit-mus-1-1":             {starts: 2, expas: 11, centerBases: 1, everyStartHasNatural: true, chokedStarts: 2},
	"m-atch-point-remastered": {starts: 2, expas: 8, centerBases: 0, everyStartHasNatural: true, chokedStarts: 2},
	"metropolis-1-1":          {starts: 4, expas: 9, centerBases: 1, everyStartHasNatural: true, chokedStarts: 4},
	"neo-sylphid-3-2":         {starts: 3, expas: 12, centerBases: 0, everyStartHasNatural: true, chokedStarts: 3},
	"o-dyssey-re-2-0":         {starts: 2, expas: 12, centerBases: 0, everyStartHasNatural: true, chokedStarts: 2},
	"octagon-1-0":             {starts: 4, expas: 12, centerBases: 0, everyStartHasNatural: true, chokedStarts: 4},
	"p-ole-s-tar-1-1":         {starts: 4, expas: 12, centerBases: 0, everyStartHasNatural: true, chokedStarts: 4},
	"polyp-oid-1-32":          {starts: 4, expas: 12, centerBases: 0, everyStartHasNatural: true, chokedStarts: 4},
	"polyp-oid-1-75":          {starts: 4, expas: 12, centerBases: 0, everyStartHasNatural: true, chokedStarts: 4},
	"r-a-d-e-o-n-1-2":         {starts: 4, expas: 10, centerBases: 0, everyStartHasNatural: true, chokedStarts: 4},
	"retro-1-2":               {starts: 4, expas: 9, centerBases: 1, everyStartHasNatural: true, chokedStarts: 4},
}

func TestSemanticInvariants(t *testing.T) {
	results := analyzeCorpus(t)

	for key := range results {
		if _, ok := expected[key]; !ok {
			t.Errorf("corpus map %q has no invariant entry; add one to `expected`", key)
		}
	}

	for key, want := range expected {
		want := want
		out, ok := results[key]
		if !ok {
			t.Errorf("%s: in invariant table but no replay produced it", key)
			continue
		}
		t.Run(key, func(t *testing.T) {
			if got := len(out.Result.Starts); got != want.starts {
				t.Errorf("starts = %d, want %d", got, want.starts)
			}
			if got := len(out.Result.Expas); got != want.expas {
				t.Errorf("expas = %d, want %d", got, want.expas)
			}
			if got := centerBaseCount(out); got != want.centerBases {
				t.Errorf("centerBases = %d, want %d", got, want.centerBases)
			}
			if want.everyStartHasNatural && !everyStartHasNatural(out.Result) {
				t.Errorf("not every start has a natural expansion")
			}
			if got := startsChokedToNatural(out.Result); got != want.chokedStarts {
				t.Errorf("chokedStarts = %d, want %d", got, want.chokedStarts)
			}
		})
	}
}

// Work items live here as a skipped test asserting the target, then graduate into
// `expected` once implemented. The 1-3/1-4 "two center bases should be one" target
// was implemented via basedetect.mergeCenterClusters and now lives in `expected`.
