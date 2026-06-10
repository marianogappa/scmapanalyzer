// Package regression is a test-only harness that characterizes the output of
// replaymap.Analyze across a corpus of real replays, so the detection algorithm
// can be evolved (and simplified) without silently regressing existing maps.
//
// It has two tiers:
//   - Semantic invariants (counts, single center base, naturals): a hand-written
//     spec in invariants_test.go. These are the things that are simply right or
//     wrong and must never regress. Change them only on purpose, in review.
//   - Geometry golden files (per-base center + polygon, compared with tolerance):
//     regenerated with `go test ./internal/regression -update`. These absorb
//     harmless vertex jitter via an IoU threshold but catch regions that move
//     or collapse.
package regression

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/marianogappa/scmapanalyzer/internal/model"
	"github.com/marianogappa/scmapanalyzer/internal/replay"
)

const (
	replaysDir = "../../replays"
	goldenDir  = "testdata/golden"
)

// mapKey mirrors cmd/replaymapanalyzer's normalizeKey so corpus keys line up
// with the filenames that tool already produces.
func mapKey(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	if v == "" {
		return ""
	}
	var b strings.Builder
	lastDash := false
	for _, r := range v {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// discoverCorpus walks the replays directory, parses each .rep, and returns one
// metadata per unique map (first replay wins, deterministic by sorted path).
func discoverCorpus() (map[string]*model.MapMetadata, error) {
	var files []string
	err := filepath.WalkDir(replaysDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() && strings.EqualFold(filepath.Ext(path), ".rep") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)

	out := make(map[string]*model.MapMetadata)
	for _, rp := range files {
		meta, parseErr := replay.ParseMapMetadata(rp)
		if parseErr != nil || meta == nil {
			continue
		}
		key := mapKey(meta.MapName)
		if key == "" {
			key = mapKey(meta.MapDataName)
		}
		if key == "" {
			continue
		}
		if _, ok := out[key]; !ok {
			out[key] = meta
		}
	}
	return out, nil
}
