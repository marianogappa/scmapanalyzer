package scmapanalyzer

import (
	"strings"
	"unicode"
)

// NormalizeMapKey strips StarCraft replay formatting controls and builds a
// lowercase, punctuation-insensitive key suitable for matching map names.
func NormalizeMapKey(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 0x20 {
			continue
		}
		switch {
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			b.WriteRune(unicode.ToLower(r))
		case unicode.IsSpace(r), r == '_', r == '.', r == '-', r == '/':
			b.WriteByte('-')
		default:
			if unicode.Is(unicode.So, r) {
				continue
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	return out
}

func levenshtein(a, b string) int {
	ar, br := []rune(a), []rune(b)
	if len(ar) == 0 {
		return len(br)
	}
	if len(br) == 0 {
		return len(ar)
	}
	prev := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	cur := make([]int, len(br)+1)
	for i := 1; i <= len(ar); i++ {
		cur[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 0
			if ar[i-1] != br[j-1] {
				cost = 1
			}
			cur[j] = min3(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(br)]
}

func min3(a, b, c int) int {
	if a <= b && a <= c {
		return a
	}
	if b <= c {
		return b
	}
	return c
}

// mapNameSimilarity is 1.0 for equal keys and decreases with edit distance.
func mapNameSimilarity(a, b string) float64 {
	a = NormalizeMapKey(a)
	b = NormalizeMapKey(b)
	if a == b {
		return 1
	}
	if a == "" || b == "" {
		return 0
	}
	d := levenshtein(a, b)
	mx := intMax(len([]rune(a)), len([]rune(b)))
	if mx == 0 {
		return 1
	}
	return 1 - float64(d)/float64(mx)
}

func intMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}
