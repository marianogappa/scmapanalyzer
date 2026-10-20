package scmapanalyzer

import (
	"bytes"
	"errors"
	"image/color"
	"strconv"
	"testing"

	"github.com/icza/screp/rep/repcore"
)

func TestUnitOrBuildingImagePNG_marine(t *testing.T) {
	b, err := UnitOrBuildingImagePNG("Terran Marine")
	if err != nil {
		t.Fatal(err)
	}
	if len(b) < 24 || string(b[1:4]) != "PNG" {
		t.Fatalf("expected PNG signature, got %d bytes", len(b))
	}
}

func TestUnitOrBuildingImagePNG_numericID(t *testing.T) {
	b, err := UnitOrBuildingImagePNG("0")
	if err != nil {
		t.Fatal(err)
	}
	if len(b) < 24 || string(b[1:4]) != "PNG" {
		t.Fatalf("expected PNG signature, got %d bytes", len(b))
	}
}

func TestUnitOrBuildingImagePNG_unknownName(t *testing.T) {
	_, err := UnitOrBuildingImagePNG("Totally Fictional Unit Xyz")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMapImageJPEGFromScrepReplay_nil(t *testing.T) {
	_, err := MapImageJPEGFromScrepReplay(nil, MapImageOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUnitOrBuildingImage_teamColorTintsOnlyMaskedPixels(t *testing.T) {
	base, mask, err := UnitOrBuildingImageLayers("Terran Marine", UnitVariantDefault)
	if err != nil {
		t.Fatal(err)
	}
	if mask == nil {
		t.Fatal("expected a team-color mask for the marine")
	}
	tinted, err := UnitOrBuildingImage("Terran Marine", UnitImageOptions{TeamColor: TeamColorFromScrep(repcore.ColorRed)})
	if err != nil {
		t.Fatal(err)
	}
	changed := 0
	for i, m := range mask.Pix {
		same := bytes.Equal(base.Pix[i*4:i*4+4], tinted.Pix[i*4:i*4+4])
		if m == 0 && !same {
			t.Fatalf("pixel %d changed outside the mask", i)
		}
		if !same {
			changed++
		}
	}
	if changed == 0 {
		t.Fatal("expected some pixels to be tinted")
	}
}

func TestUnitOrBuildingImage_burrowedLurker(t *testing.T) {
	burrowed, err := UnitOrBuildingImage("Zerg Lurker", UnitImageOptions{Variant: UnitVariantBurrowed})
	if err != nil {
		t.Fatal(err)
	}
	standing, err := UnitOrBuildingImage("Zerg Lurker", UnitImageOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if burrowed.Bounds() == standing.Bounds() {
		t.Fatal("expected burrowed lurker to differ from the standing sprite")
	}
}

func TestUnitOrBuildingImage_variantNotAvailable(t *testing.T) {
	_, err := UnitOrBuildingImage("Zerg Hydralisk", UnitImageOptions{Variant: UnitVariantBurrowed})
	if !errors.Is(err, ErrUnitVariantNotAvailable) {
		t.Fatalf("expected ErrUnitVariantNotAvailable, got %v", err)
	}
}

func TestTeamColorFromScrep(t *testing.T) {
	got := TeamColorFromScrep(repcore.ColorRed)
	want := color.NRGBA{R: uint8(repcore.ColorRed.RGB >> 16), G: uint8(repcore.ColorRed.RGB >> 8), B: uint8(repcore.ColorRed.RGB), A: 255}
	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestUnitImageAnchor_everySpriteHasOneInsideIt(t *testing.T) {
	for id, name := range hdSpriteUnitNames {
		img, err := UnitOrBuildingImage(name, UnitImageOptions{})
		if err != nil {
			t.Fatal(err)
		}
		x, y, ok := UnitImageAnchor(strconv.Itoa(int(id)), UnitImageOptions{})
		if !ok {
			t.Fatalf("%s (%d): no anchor", name, id)
		}
		b := img.Bounds()
		if x < -b.Dx() || x > 2*b.Dx() || y < -b.Dy() || y > 2*b.Dy() {
			t.Fatalf("%s (%d): anchor (%d,%d) far outside %v", name, id, x, y, b)
		}
	}
}

func TestUnitShadowImage(t *testing.T) {
	sh, err := UnitOrBuildingImage("Terran Wraith", UnitImageOptions{Shadow: true})
	if err != nil {
		t.Fatal(err)
	}
	_, sy, ok := UnitImageAnchor("Terran Wraith", UnitImageOptions{Shadow: true})
	if !ok {
		t.Fatal("expected a shadow anchor")
	}
	if sy >= 0 {
		t.Fatalf("an air unit's shadow should sit below it, got anchor y %d", sy)
	}
	for i := 0; i < len(sh.Pix); i += 4 {
		if sh.Pix[i] != 0 || sh.Pix[i+1] != 0 || sh.Pix[i+2] != 0 || sh.Pix[i+3] > 128 {
			t.Fatalf("shadow pixel %v is not translucent black", sh.Pix[i:i+4])
		}
	}
	if _, err := UnitShadowImage("Protoss Archon", UnitVariantDefault); !errors.Is(err, ErrShadowNotAvailable) {
		t.Fatalf("expected ErrShadowNotAvailable, got %v", err)
	}
	if _, _, ok := UnitImageAnchor("Protoss Archon", UnitImageOptions{Shadow: true}); ok {
		t.Fatal("expected no shadow anchor for the archon")
	}
}

func TestCompositeUnitsHaveTeamColorAndGameScale(t *testing.T) {
	for name, maxH := range map[string]int{"Terran Goliath": 110, "Protoss Dark Templar": 80, "Protoss Archon": 200, "Protoss Dark Archon": 180, "Terran Siege Tank": 120} {
		img, mask, err := UnitOrBuildingImageLayers(name, UnitVariantDefault)
		if err != nil {
			t.Fatal(err)
		}
		if mask == nil {
			t.Fatalf("%s: expected a team-color mask", name)
		}
		if h := img.Bounds().Dy(); h > maxH {
			t.Fatalf("%s: %d px tall, want at most %d", name, h, maxH)
		}
	}
}

func TestCreepPNGs(t *testing.T) {
	for _, f := range []func(string) ([]byte, error){CreepTexturePNG, CreepEdgesPNG} {
		b, err := f("badlands")
		if err != nil {
			t.Fatal(err)
		}
		if string(b[1:4]) != "PNG" {
			t.Fatal("expected PNG")
		}
	}
	if CreepEdgeFrame(CreepN|CreepNE|CreepNW) == 0 {
		t.Fatal("expected an edge below a creep row")
	}
}
