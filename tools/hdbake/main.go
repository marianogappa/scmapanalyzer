// Command hdbake builds the unit sprites embedded in lib/scmapanalyzer/hd_units from a local
// StarCraft: Remastered install, reading HD2/anim/main_NNN.anim and the arr/*.dat, images.tbl,
// iscript.bin and LO offset files straight from its CASC storage. It also bakes each tileset's
// creep edge frames for internal/mapgfx.
//
// Most sprites were baked earlier from a single HD2 frame and then hand-tuned, so they are kept
// as they are: hdbake finds the frame and crop each one came from and records where the unit's
// position falls inside it (its anchor), plus the unit's shadow for that frame. Units the game
// draws from several images are composed here instead (see composites). Run from the repo root:
//
//	go run ./tools/hdbake -sc /Applications/StarCraft
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/marianogappa/scmapanalyzer/internal/qoi"
	"github.com/marianogappa/scmapanalyzer/tools/hdbake/casc"
)

// facing is the direction composites face: 17 directions per frame set, 0 = north, 8 = east,
// 16 = south. Most of the hand-picked single-frame sprites face this way too.
const facing = 13

// layer is one image of a unit drawn at offset (in HD2 pixels) from the unit's position.
type layer struct {
	image, frame int
	offset       image.Point
}

type composite struct {
	layers []layer
	// shadowOf is the layer whose shadow image (and frame) the unit casts; -1 for none.
	shadowOf int
}

func composites(d *gameData) (map[int]composite, error) {
	turret := func(hull, hullFrame, turretImage, turretFrame int) ([]layer, error) {
		x, y, err := d.loOffset(hull, hullFrame)
		if err != nil {
			return nil, err
		}
		return []layer{{image: hull, frame: hullFrame}, {image: turretImage, frame: turretFrame, offset: image.Pt(2*x, 2*y)}}, nil
	}
	const standing = 7 * 17 // the goliath's idle frame set, from its iscript init (playfram 0x77)
	goliath, err := turret(234, standing+facing, 235, standing+facing)
	if err != nil {
		return nil, err
	}
	tank, err := turret(250, facing, 251, facing)
	if err != nil {
		return nil, err
	}
	siege, err := turret(253, 5, 254, facing) // hull frame 5 is fully sieged; it does not turn
	if err != nil {
		return nil, err
	}
	return map[int]composite{
		3:  {layers: goliath, shadowOf: 0},
		5:  {layers: tank, shadowOf: 0},
		30: {layers: siege, shadowOf: 0},
		// Dark templar: images.dat id 933 has no HD2 anim; image 129 shares its grp (dtemplar.grp).
		// Frame set 13 is its iscript init pose. DTs cast no shadow.
		61: {layers: []layer{{image: 129, frame: 13*17 + facing}}, shadowOf: -1},
		// Archon: energy ball (134), the merged templar (135, imgol) and team-colored arcs (136, imgol).
		68: {layers: []layer{{image: 134}, {image: 135, frame: 10*17 + facing}, {image: 136}}, shadowOf: -1},
		63: {layers: []layer{{image: 925}, {image: 926, frame: facing}, {image: 927}}, shadowOf: -1},
	}, nil
}

func main() {
	scDir := flag.String("sc", "/Applications/StarCraft", "StarCraft: Remastered install directory")
	unitsDir := flag.String("units", "lib/scmapanalyzer/hd_units", "directory of embedded u<ID>.qoi sprites")
	anchorsFile := flag.String("anchors", "lib/scmapanalyzer/hd_unit_anchors.go", "generated anchors Go file")
	tilesetsDir := flag.String("tilesets", "internal/mapgfx/data/tilesets", "directory for creep_edges.png per tileset")
	flag.Parse()
	if err := run(*scDir, *unitsDir, *anchorsFile, *tilesetsDir); err != nil {
		fmt.Fprintln(os.Stderr, "hdbake:", err)
		os.Exit(1)
	}
}

type baker struct {
	st    *casc.Storage
	data  *gameData
	anims map[int]*anim
}

func run(scDir, unitsDir, anchorsFile, tilesetsDir string) error {
	st, err := casc.Open(scDir)
	if err != nil {
		return err
	}
	defer st.Close()
	if err := bakeCreepEdges(st, tilesetsDir); err != nil {
		return err
	}
	data, err := loadGameData(st.ReadFile)
	if err != nil {
		return err
	}
	b := &baker{st: st, data: data, anims: map[int]*anim{}}
	comps, err := composites(data)
	if err != nil {
		return err
	}
	names, err := spriteNames(unitsDir)
	if err != nil {
		return err
	}
	anchors := map[string]image.Point{}
	for _, name := range names {
		unit, variant := parseSpriteName(name)
		var s *sprite
		var shadowFrom *layer
		if c, ok := comps[unit]; ok && variant == "" {
			if s, err = b.render(c.layers); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			if err := writeSprite(unitsDir, name, s); err != nil {
				return err
			}
			if c.shadowOf >= 0 {
				shadowFrom = &c.layers[c.shadowOf]
			}
			fmt.Printf("%s: composed %dx%d anchor %v\n", name, s.pix.Bounds().Dx(), s.pix.Bounds().Dy(), s.anchor)
		} else {
			existing, err := readQOI(filepath.Join(unitsDir, name+".qoi"))
			if err != nil {
				return err
			}
			l, anchor, err := b.locate(data.unitImage(unit), existing)
			if err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			s = &sprite{pix: existing, anchor: anchor}
			if variant == "" {
				shadowFrom = &l
			}
			fmt.Printf("%s: image %d frame %d anchor %v\n", name, l.image, l.frame, anchor)
		}
		anchors[name] = s.anchor
		if shadowFrom == nil {
			continue
		}
		sh, ok, err := b.shadow(*shadowFrom)
		if err != nil {
			return fmt.Errorf("%s shadow: %w", name, err)
		}
		if !ok {
			continue
		}
		if err := writeShadow(unitsDir, name, sh); err != nil {
			return err
		}
		anchors[name+".shadow"] = sh.anchor
	}
	return writeAnchors(anchorsFile, anchors)
}

var spriteFile = regexp.MustCompile(`^(u(\d+)(?:_[a-z]+)?)\.qoi$`)

func spriteNames(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if m := spriteFile.FindStringSubmatch(e.Name()); m != nil {
			names = append(names, m[1])
		}
	}
	return names, nil
}

func parseSpriteName(name string) (unit int, variant string) {
	id, variant, _ := strings.Cut(strings.TrimPrefix(name, "u"), "_")
	unit, _ = strconv.Atoi(id)
	return unit, variant
}

// anim loads an image's HD2 anim. Several images.dat ids share one grp but only one of them has
// an anim (e.g. the dark templar's 933 and 129), so a missing anim falls back to a sibling's. Grp
// names are compared ignoring case: the lurker's 921 uses "Zlurker.grp" and 920 "zLurker.grp".
func (b *baker) anim(imageID int) (*anim, error) {
	if a, ok := b.anims[imageID]; ok {
		return a, nil
	}
	candidates := []int{imageID}
	grp := b.data.imagesTbl[b.data.images[imageID].grp]
	for i, im := range b.data.images {
		if i != imageID && strings.EqualFold(b.data.imagesTbl[im.grp], grp) {
			candidates = append(candidates, i)
		}
	}
	for _, id := range candidates {
		raw, err := b.st.ReadFile(fmt.Sprintf("HD2/anim/main_%03d.anim", id))
		if err != nil {
			continue
		}
		a, err := parseAnim(raw)
		if err != nil {
			return nil, fmt.Errorf("main_%03d.anim: %w", id, err)
		}
		b.anims[imageID] = a
		return a, nil
	}
	return nil, fmt.Errorf("no HD2 anim for image %d", imageID)
}

// locate finds the frame and crop an existing sprite was cut from, and returns its anchor.
func (b *baker) locate(imageID int, want *image.NRGBA) (layer, image.Point, error) {
	a, err := b.anim(imageID)
	if err != nil {
		return layer{}, image.Point{}, err
	}
	diffuse := a.layers["diffuse"]
	size := want.Bounds().Size()
	for i := range a.frames {
		f, err := a.frame(i)
		if err != nil {
			continue
		}
		for y := f.rect.Min.Y; y+size.Y <= f.rect.Max.Y; y++ {
			for x := f.rect.Min.X; x+size.X <= f.rect.Max.X; x++ {
				if matchesAt(diffuse, image.Pt(x, y), want) {
					return layer{image: imageID, frame: i}, f.rect.Min.Add(f.origin).Sub(image.Pt(x, y)), nil
				}
			}
		}
	}
	return layer{}, image.Point{}, fmt.Errorf("no frame of image %d matches the existing sprite", imageID)
}

func matchesAt(atlas *image.NRGBA, at image.Point, want *image.NRGBA) bool {
	b := want.Bounds()
	for y := range b.Dy() {
		wr := want.Pix[want.PixOffset(0, y):]
		ar := atlas.Pix[atlas.PixOffset(at.X, at.Y+y):]
		for x := range b.Dx() {
			w, g := wr[x*4:x*4+4], ar[x*4:x*4+4]
			if w[3] != g[3] || (w[3] != 0 && !bytes.Equal(w[:3], g[:3])) {
				return false
			}
		}
	}
	return true
}

func (b *baker) shadow(of layer) (*shadowSprite, bool, error) {
	shadowImage, dx, dy, ok := b.data.shadowOf(of.image)
	if !ok {
		return nil, false, nil
	}
	a, err := b.anim(shadowImage)
	if err != nil {
		return nil, false, err
	}
	frame := of.frame
	if frame >= len(a.frames) {
		// A few buildings (e.g. the hatchery) were baked from a frame past their shadow's last.
		frame = 0
	}
	s, err := b.render([]layer{{image: shadowImage, frame: frame, offset: of.offset.Add(image.Pt(2*dx, 2*dy))}})
	if err != nil {
		return nil, false, err
	}
	cov := image.NewGray(s.pix.Bounds())
	for i := range cov.Pix {
		cov.Pix[i] = s.pix.Pix[i*4+3]
	}
	return &shadowSprite{coverage: cov, anchor: s.anchor}, true, nil
}

type sprite struct {
	pix  *image.NRGBA
	mask *image.Gray
	// anchor is where the unit's position falls inside pix.
	anchor image.Point
}

type shadowSprite struct {
	coverage *image.Gray
	anchor   image.Point
}

// trimAlpha drops the archons' faint outer glow, which would otherwise pad their frames by ~40%.
const trimAlpha = 8

// render draws layers bottom to top around the unit's position and trims the result.
func (b *baker) render(layers []layer) (*sprite, error) {
	type placed struct {
		a    *anim
		f    animFrame
		dest image.Rectangle
	}
	var ps []placed
	var bounds image.Rectangle
	for _, l := range layers {
		a, err := b.anim(l.image)
		if err != nil {
			return nil, err
		}
		f, err := a.frame(l.frame)
		if err != nil {
			return nil, fmt.Errorf("image %d: %w", l.image, err)
		}
		dest := f.rect.Sub(f.rect.Min).Sub(f.origin).Add(l.offset)
		ps = append(ps, placed{a, f, dest})
		bounds = bounds.Union(dest)
	}
	n := bounds.Dx() * bounds.Dy()
	rgba := make([][4]float64, n)
	team := make([]float64, n)
	for _, p := range ps {
		diffuse, tc := p.a.layers["diffuse"], p.a.layers["teamcolor"]
		for y := p.dest.Min.Y; y < p.dest.Max.Y; y++ {
			for x := p.dest.Min.X; x < p.dest.Max.X; x++ {
				sx, sy := p.f.rect.Min.X+x-p.dest.Min.X, p.f.rect.Min.Y+y-p.dest.Min.Y
				c := diffuse.NRGBAAt(sx, sy)
				a := float64(c.A) / 255
				if a == 0 {
					continue
				}
				m := 0.0
				if tc != nil {
					m = float64(tc.NRGBAAt(sx, sy).R) / 255
				}
				i := (y-bounds.Min.Y)*bounds.Dx() + (x - bounds.Min.X)
				d := rgba[i]
				outA := a + d[3]*(1-a)
				for k, v := range [3]uint8{c.R, c.G, c.B} {
					rgba[i][k] = (float64(v)*a + d[k]*d[3]*(1-a)) / outA
				}
				team[i] = (m*a + team[i]*d[3]*(1-a)) / outA
				rgba[i][3] = outA
			}
		}
	}
	trim := image.Rectangle{Min: bounds.Max, Max: bounds.Min}
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if rgba[(y-bounds.Min.Y)*bounds.Dx()+(x-bounds.Min.X)][3]*255 >= trimAlpha {
				trim = trim.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	if trim.Empty() {
		return nil, fmt.Errorf("layers %v render empty", layers)
	}
	pix := image.NewNRGBA(image.Rect(0, 0, trim.Dx(), trim.Dy()))
	mask := image.NewGray(pix.Bounds())
	hasMask := false
	for y := trim.Min.Y; y < trim.Max.Y; y++ {
		for x := trim.Min.X; x < trim.Max.X; x++ {
			i := (y-bounds.Min.Y)*bounds.Dx() + (x - bounds.Min.X)
			o := pix.PixOffset(x-trim.Min.X, y-trim.Min.Y)
			for k := range 4 {
				scale := 255.0
				if k < 3 {
					scale = 1
				}
				pix.Pix[o+k] = uint8(min(255, rgba[i][k]*scale+0.5))
			}
			if pix.Pix[o+3] != 0 {
				m := uint8(team[i]*255 + 0.5)
				mask.Pix[mask.PixOffset(x-trim.Min.X, y-trim.Min.Y)] = m
				hasMask = hasMask || m != 0
			}
		}
	}
	if !hasMask {
		mask = nil
	}
	return &sprite{pix: pix, mask: mask, anchor: trim.Min.Mul(-1)}, nil
}

func readQOI(path string) (*image.NRGBA, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return qoi.DecodeToNRGBA(b)
}

func writeSprite(dir, name string, s *sprite) error {
	b := s.pix.Bounds()
	q, err := qoi.EncodeRGBA(b.Dx(), b.Dy(), s.pix.Pix)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, name+".qoi"), q, 0o644); err != nil {
		return err
	}
	maskPath := filepath.Join(dir, name+".mask.png")
	if s.mask == nil {
		if err := os.Remove(maskPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return writePNG(maskPath, s.mask)
}

func writeShadow(dir, name string, s *shadowSprite) error {
	return writePNG(filepath.Join(dir, name+".shadow.png"), s.coverage)
}

func writePNG(path string, img image.Image) error {
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(&buf, img); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func writeAnchors(path string, anchors map[string]image.Point) error {
	keys := make([]string, 0, len(anchors))
	for k := range anchors {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b string) int {
		ua, _ := parseSpriteName(strings.TrimSuffix(a, ".shadow"))
		ub, _ := parseSpriteName(strings.TrimSuffix(b, ".shadow"))
		if ua != ub {
			return ua - ub
		}
		return strings.Compare(a, b)
	})
	var buf bytes.Buffer
	buf.WriteString("// Code generated by tools/hdbake; DO NOT EDIT.\n\npackage scmapanalyzer\n\nimport \"image\"\n\n")
	buf.WriteString("// hdSpriteAnchors maps an embedded sprite (hd_units/<key>.qoi, or <key>.png for \".shadow\" keys)\n")
	buf.WriteString("// to the pixel where the unit's position falls inside it.\n")
	buf.WriteString("var hdSpriteAnchors = map[string]image.Point{\n")
	for _, k := range keys {
		fmt.Fprintf(&buf, "\t%q: {X: %d, Y: %d},\n", k, anchors[k].X, anchors[k].Y)
	}
	buf.WriteString("}\n")
	src, err := format.Source(buf.Bytes())
	if err != nil {
		return err
	}
	return os.WriteFile(path, src, 0o644)
}
