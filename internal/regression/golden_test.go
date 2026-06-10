package regression

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/marianogappa/scmapanalyzer/replaymap"
)

var update = flag.Bool("update", false, "regenerate golden geometry files")

// Tolerances for the geometry tier. Loose enough to absorb vertex jitter from a
// legitimately-improved algorithm, tight enough to catch a region that actually
// moved or collapsed. Tune in one place.
const (
	minIoU             = 0.80 // per-base polygon overlap vs golden
	maxCenterDistTiles = 16.0 // per-base center drift vs golden (minitiles)
)

func analyzeCorpus(t *testing.T) map[string]*replaymap.AnalyzeOutput {
	t.Helper()
	corpus, err := discoverCorpus()
	if err != nil {
		t.Fatalf("discover corpus: %v", err)
	}
	if len(corpus) == 0 {
		t.Fatalf("no replays found under %s", replaysDir)
	}
	out := make(map[string]*replaymap.AnalyzeOutput, len(corpus))
	for key, meta := range corpus {
		res, err := replaymap.Analyze(meta)
		if err != nil {
			t.Errorf("%s: analyze: %v", key, err)
			continue
		}
		out[key] = res
	}
	return out
}

func TestGoldenGeometry(t *testing.T) {
	results := analyzeCorpus(t)

	if *update {
		if err := os.MkdirAll(goldenDir, 0o755); err != nil {
			t.Fatalf("mkdir golden: %v", err)
		}
		for key, out := range results {
			b, err := json.MarshalIndent(out.Result, "", "  ")
			if err != nil {
				t.Fatalf("%s: marshal: %v", key, err)
			}
			path := filepath.Join(goldenDir, key+".json")
			if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
				t.Fatalf("%s: write golden: %v", key, err)
			}
		}
		t.Logf("regenerated %d golden files in %s", len(results), goldenDir)
		return
	}

	for _, key := range sortedKeys(results) {
		key, out := key, results[key]
		t.Run(key, func(t *testing.T) {
			golden, err := loadGolden(key)
			if err != nil {
				t.Fatalf("load golden (run with -update to create): %v", err)
			}
			compareGeometry(t, golden, out.Result)
		})
	}
}

func compareGeometry(t *testing.T, golden, got *replaymap.Result) {
	t.Helper()
	gotByName := indexByName(got)
	matched := map[string]bool{}
	for _, gb := range allBasesOut(golden) {
		cur, ok := gotByName[gb.Name]
		if !ok {
			t.Errorf("base %q present in golden but missing now", gb.Name)
			continue
		}
		matched[gb.Name] = true
		if d := centerDist(gb.CenterTile, cur.CenterTile); d > maxCenterDistTiles {
			t.Errorf("base %q center drifted %.1f minitiles (max %.0f): golden=%v now=%v",
				gb.Name, d, maxCenterDistTiles, gb.CenterTile, cur.CenterTile)
		}
		if iou := polygonIoU(gb.PolygonVertices, cur.PolygonVertices); iou < minIoU {
			t.Errorf("base %q polygon IoU %.3f < %.2f", gb.Name, iou, minIoU)
		}
	}
	for _, cb := range allBasesOut(got) {
		if !matched[cb.Name] {
			t.Errorf("base %q present now but not in golden", cb.Name)
		}
	}
}

func loadGolden(key string) (*replaymap.Result, error) {
	b, err := os.ReadFile(filepath.Join(goldenDir, key+".json"))
	if err != nil {
		return nil, err
	}
	var r replaymap.Result
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func indexByName(r *replaymap.Result) map[string]replaymap.BasePolygon {
	m := make(map[string]replaymap.BasePolygon)
	for _, b := range allBasesOut(r) {
		m[b.Name] = b
	}
	return m
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// TestCorpusSummary is informational: run `go test -run CorpusSummary -v` to
// see the counts that the invariant spec should encode.
func TestCorpusSummary(t *testing.T) {
	results := analyzeCorpus(t)
	for _, key := range sortedKeys(results) {
		out := results[key]
		fmt.Printf("%-30s starts=%d expas=%d centerBases=%d clock0=%d everyStartNatural=%v\n",
			key, len(out.Result.Starts), len(out.Result.Expas),
			centerBaseCount(out), clock0Count(out.Result), everyStartHasNatural(out.Result))
	}
}

func clock0Count(r *replaymap.Result) int {
	n := 0
	for _, b := range r.Expas {
		if b.Clock == 0 {
			n++
		}
	}
	return n
}

func everyStartHasNatural(r *replaymap.Result) bool {
	for _, s := range r.Starts {
		if s.NaturalExpansion == "" {
			return false
		}
	}
	return len(r.Starts) > 0
}
