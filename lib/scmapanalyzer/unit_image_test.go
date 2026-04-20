package scmapanalyzer

import "testing"

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

func TestMapImagePNGFromScrepReplay_nil(t *testing.T) {
	_, err := MapImagePNGFromScrepReplay(nil)
	if err == nil {
		t.Fatal("expected error")
	}
}
