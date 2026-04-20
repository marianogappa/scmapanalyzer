package scmapanalyzer

import (
	"bytes"
	"errors"
	"fmt"
	"image/png"
	"io/fs"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/marianogappa/scmapanalyzer/internal/qoi"
)

var hdNameToID map[string]uint16

func init() {
	hdNameToID = make(map[string]uint16, len(hdSpriteUnitNames))
	ids := make([]uint16, 0, len(hdSpriteUnitNames))
	for id := range hdSpriteUnitNames {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		name := hdSpriteUnitNames[id]
		key := normalizeUnitOrBuildingName(name)
		if key == "" {
			continue
		}
		if _, exists := hdNameToID[key]; exists {
			continue
		}
		hdNameToID[key] = id
	}
}

func normalizeUnitOrBuildingName(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	var b strings.Builder
	lastSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !lastSpace && b.Len() > 0 {
				b.WriteByte(' ')
				lastSpace = true
			}
			continue
		}
		lastSpace = false
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

func resolveUnitOrBuildingID(name string) (uint16, error) {
	raw := strings.TrimSpace(name)
	if raw == "" {
		return 0, errors.New("unit or building name is empty")
	}
	if isAllDecimalDigits(raw) {
		n, err := strconv.ParseUint(raw, 10, 16)
		if err != nil {
			return 0, fmt.Errorf("parse unit type id: %w", err)
		}
		id := uint16(n)
		if _, ok := hdSpriteUnitNames[id]; !ok {
			return 0, fmt.Errorf("no baked sprite for unit type id %d", id)
		}
		return id, nil
	}
	key := normalizeUnitOrBuildingName(raw)
	id, ok := hdNameToID[key]
	if !ok {
		return 0, fmt.Errorf("unknown unit or building name %q", name)
	}
	return id, nil
}

func isAllDecimalDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func hdQOIPath(unitID uint16) string {
	return fmt.Sprintf("hd_units/u%d.qoi", unitID)
}

// UnitOrBuildingImagePNG returns a PNG for a Brood War unit or building. Name is either a
// decimal unit type id (e.g. "0") or the display name in [hdSpriteUnitNames] (e.g. "Terran Marine",
// case-insensitive). When two ids share the same display name, the lower id wins.
func UnitOrBuildingImagePNG(name string) ([]byte, error) {
	id, err := resolveUnitOrBuildingID(name)
	if err != nil {
		return nil, err
	}
	rel := hdQOIPath(id)
	qb, err := fs.ReadFile(hdQoiFiles, rel)
	if err != nil {
		return nil, fmt.Errorf("read hd unit qoi: %w", err)
	}
	img, err := qoi.DecodeToNRGBA(qb)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
