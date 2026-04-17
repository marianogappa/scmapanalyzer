package replay

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/icza/screp/rep/repcore"
)

type TileSetMeta struct {
	ID   int
	Name string
	Key  string
}

func TileSetMetadata(id int, tileSetMissing bool) TileSetMeta {
	name := tileSetNameByID(id)
	if tileSetMissing {
		name = name + "-missing-era"
	}
	return TileSetMeta{
		ID:   id,
		Name: name,
		Key:  fmt.Sprintf("%02d-%s", id, slugify(name)),
	}
}

func tileSetNameByID(id int) string {
	ts := repcore.TileSetByID(uint16(id))
	if ts == nil {
		return "unknown"
	}
	name := strings.TrimSpace(strings.ToLower(ts.Name))
	if name == "" {
		return "unknown"
	}
	return name
}

func slugify(s string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "unknown"
	}
	return out
}
