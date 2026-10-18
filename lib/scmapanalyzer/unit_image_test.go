package scmapanalyzer

import (
	"bytes"
	"errors"
	"image/color"
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
