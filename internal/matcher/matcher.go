package matcher

import (
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/marianogappa/scmapanalyzer/internal/model"
)

func NormalizeName(s string) string {
	s = strings.ToLower(s)
	s = stripControl(s)
	replacer := strings.NewReplacer(
		"_", " ",
		"-", " ",
		"(", "",
		")", "",
		"[", "",
		"]", "",
		"{", "",
		"}", "",
		".", "",
		",", "",
		"'", "",
		"\"", "",
		":", "",
		";", "",
		"/", "",
		"\\", "",
		"&", " and ",
	)
	s = replacer.Replace(s)
	tokens := strings.Fields(s)
	filtered := make([]string, 0, len(tokens))
	for _, t := range tokens {
		t = keepAlphaNum(t)
		if t == "" {
			continue
		}
		if isVersionToken(t) {
			continue
		}
		filtered = append(filtered, t)
	}
	return strings.Join(filtered, " ")
}

func ScoreName(query string, candidate string) float64 {
	q := NormalizeName(query)
	c := NormalizeName(candidate)
	if q == "" || c == "" {
		return 0
	}
	if q == c {
		return 1
	}
	qCompact := strings.ReplaceAll(q, " ", "")
	cCompact := strings.ReplaceAll(c, " ", "")
	if qCompact == cCompact {
		return 0.99
	}
	if qCompact != "" && cCompact != "" && (strings.Contains(qCompact, cCompact) || strings.Contains(cCompact, qCompact)) {
		return 0.94
	}

	edit := 1 - (float64(levenshtein(qCompact, cCompact)) / float64(max(len([]rune(qCompact)), len([]rune(cCompact)))))
	token := tokenSimilarity(q, c)
	prefix := 0.0
	if strings.HasPrefix(qCompact, cCompact) || strings.HasPrefix(cCompact, qCompact) {
		prefix = 1
	}
	return 0.5*edit + 0.35*token + 0.15*prefix
}

func MatchMapImage(mapName string, mapDataName string, imagePaths []string, topN int, minScore float64) model.MatchResult {
	if topN <= 0 {
		topN = 5
	}
	var candidates []model.MatchCandidate
	queries := expandQueries(mapName, mapDataName)
	for _, path := range imagePaths {
		name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		score := 0.0
		for _, query := range queries {
			score = math.Max(score, ScoreName(query, name))
		}
		candidates = append(candidates, model.MatchCandidate{
			ImagePath: path,
			ImageName: filepath.Base(path),
			Score:     score,
		})
	}
	sort.Slice(candidates, func(i int, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})
	if len(candidates) > topN {
		candidates = candidates[:topN]
	}
	result := model.MatchResult{
		ReplayMap:  mapName,
		Candidates: candidates,
	}
	if len(candidates) == 0 {
		result.Reason = "no image candidates"
		return result
	}
	best := candidates[0]
	if best.Score < minScore {
		result.Reason = "below threshold"
		return result
	}
	result.Accepted = true
	result.Chosen = &best
	return result
}

func expandQueries(mapName string, mapDataName string) []string {
	queries := []string{mapName, mapDataName}
	aliases := map[string][]string{
		"투혼": {"fighting spirit"},
	}
	for _, raw := range []string{mapName, mapDataName} {
		clean := NormalizeName(raw)
		if extra, ok := aliases[clean]; ok {
			queries = append(queries, extra...)
		}
	}
	return queries
}

func tokenSimilarity(a string, b string) float64 {
	at := tokenSet(a)
	bt := tokenSet(b)
	if len(at) == 0 || len(bt) == 0 {
		return 0
	}
	common := 0
	for k := range at {
		if bt[k] {
			common++
		}
	}
	return float64(common) / float64(len(at)+len(bt)-common)
}

func tokenSet(s string) map[string]bool {
	out := map[string]bool{}
	for _, token := range strings.Fields(s) {
		if token == "" {
			continue
		}
		out[token] = true
	}
	return out
}

func levenshtein(a string, b string) int {
	ra := []rune(a)
	rb := []rune(b)
	if len(ra) == 0 {
		return len(rb)
	}
	if len(rb) == 0 {
		return len(ra)
	}
	prev := make([]int, len(rb)+1)
	curr := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		curr[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 0
			if ra[i-1] != rb[j-1] {
				cost = 1
			}
			curr[j] = min(
				curr[j-1]+1,
				prev[j]+1,
				prev[j-1]+cost,
			)
		}
		copy(prev, curr)
	}
	return prev[len(rb)]
}

func min(values ...int) int {
	best := values[0]
	for _, v := range values[1:] {
		if v < best {
			best = v
		}
	}
	return best
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func stripControl(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func keepAlphaNum(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) || unicode.IsLetter(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func isVersionToken(token string) bool {
	token = strings.TrimPrefix(token, "v")
	if strings.Count(token, ".") > 3 {
		return false
	}
	parts := strings.Split(token, ".")
	if len(parts) == 0 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		if _, err := strconv.Atoi(p); err != nil {
			return false
		}
	}
	return true
}
