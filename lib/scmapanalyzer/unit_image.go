package scmapanalyzer

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/icza/screp/rep/repcore"
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

// UnitVariant selects an alternate pose of a unit, such as a burrowed lurker.
type UnitVariant string

const (
	UnitVariantDefault  UnitVariant = ""
	UnitVariantBurrowed UnitVariant = "burrowed"
)

// ErrUnitVariantNotAvailable is returned when a unit has no sprite for the requested variant.
// Callers decide whether to fall back to [UnitVariantDefault]; a burrowed unit drawn standing
// would be misleading, so the library does not fall back on its own.
var ErrUnitVariantNotAvailable = errors.New("unit variant not available")

// UnitImageOptions controls [UnitOrBuildingImage].
type UnitImageOptions struct {
	// TeamColor tints the unit's team-color areas. Nil leaves them untinted.
	TeamColor color.Color
	Variant   UnitVariant
}

// TeamColorFromScrep converts a replay player color to a team color.
func TeamColorFromScrep(c *repcore.Color) color.NRGBA {
	if c == nil {
		return color.NRGBA{A: 255}
	}
	return color.NRGBA{R: uint8(c.RGB >> 16), G: uint8(c.RGB >> 8), B: uint8(c.RGB), A: 255}
}

func hdSpriteBaseName(unitID uint16, variant UnitVariant) string {
	if variant == UnitVariantDefault {
		return fmt.Sprintf("hd_units/u%d", unitID)
	}
	return fmt.Sprintf("hd_units/u%d_%s", unitID, variant)
}

// UnitOrBuildingImageLayers returns the untinted sprite and its team-color mask, for callers that
// tint on their own (e.g. in a browser). Mask values are tint strength (0 = none, 255 = full);
// mask is nil for sprites without team-color areas.
func UnitOrBuildingImageLayers(name string, variant UnitVariant) (*image.NRGBA, *image.Gray, error) {
	id, err := resolveUnitOrBuildingID(name)
	if err != nil {
		return nil, nil, err
	}
	base := hdSpriteBaseName(id, variant)
	qb, err := fs.ReadFile(hdQoiFiles, base+".qoi")
	if errors.Is(err, fs.ErrNotExist) && variant != UnitVariantDefault {
		return nil, nil, fmt.Errorf("%w: %q for %s", ErrUnitVariantNotAvailable, variant, name)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("read hd unit qoi: %w", err)
	}
	img, err := qoi.DecodeToNRGBA(qb)
	if err != nil {
		return nil, nil, err
	}
	mb, err := fs.ReadFile(hdQoiFiles, base+".mask.png")
	if errors.Is(err, fs.ErrNotExist) {
		return img, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("read hd unit mask: %w", err)
	}
	decoded, err := png.Decode(bytes.NewReader(mb))
	if err != nil {
		return nil, nil, fmt.Errorf("decode hd unit mask: %w", err)
	}
	mask, ok := decoded.(*image.Gray)
	if !ok || mask.Bounds() != img.Bounds() {
		return nil, nil, fmt.Errorf("hd unit mask for %s does not match its sprite", name)
	}
	return img, mask, nil
}

// UnitOrBuildingImage returns a unit or building sprite, tinted with opts.TeamColor.
// Name is resolved as in [UnitOrBuildingImagePNG].
func UnitOrBuildingImage(name string, opts UnitImageOptions) (*image.NRGBA, error) {
	img, mask, err := UnitOrBuildingImageLayers(name, opts.Variant)
	if err != nil {
		return nil, err
	}
	if opts.TeamColor != nil && mask != nil {
		applyTeamColor(img, mask, color.NRGBAModel.Convert(opts.TeamColor).(color.NRGBA))
	}
	return img, nil
}

// UnitOrBuildingImagePNGWithOptions is [UnitOrBuildingImage] encoded as PNG.
func UnitOrBuildingImagePNGWithOptions(name string, opts UnitImageOptions) ([]byte, error) {
	img, err := UnitOrBuildingImage(name, opts)
	if err != nil {
		return nil, err
	}
	return encodeUnitPNG(img)
}

// applyTeamColor multiplies masked pixels by twice the team color. Team-color areas in the diffuse
// layer are mid-gray, so a 2x multiply keeps their shading while staying as vivid as the game; a
// plain multiply looks muddy and a flat blend loses the shading.
func applyTeamColor(img *image.NRGBA, mask *image.Gray, team color.NRGBA) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			m := uint32(mask.GrayAt(x, y).Y)
			if m == 0 {
				continue
			}
			o := img.PixOffset(x, y)
			for c, t := range [3]uint8{team.R, team.G, team.B} {
				d := uint32(img.Pix[o+c])
				tinted := min(255, d*uint32(t)*2/255)
				img.Pix[o+c] = uint8((d*(255-m) + tinted*m) / 255)
			}
		}
	}
}

// UnitOrBuildingImagePNG returns a PNG for a Brood War unit or building. Name is either a
// decimal unit type id (e.g. "0") or the display name in [hdSpriteUnitNames] (e.g. "Terran Marine",
// case-insensitive). When two ids share the same display name, the lower id wins. Team-color
// areas are left untinted; see [UnitOrBuildingImagePNGWithOptions].
func UnitOrBuildingImagePNG(name string) ([]byte, error) {
	return UnitOrBuildingImagePNGWithOptions(name, UnitImageOptions{})
}

func encodeUnitPNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
