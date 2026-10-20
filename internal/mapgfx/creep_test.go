package mapgfx

import (
	"slices"
	"testing"
)

func TestCreepEdgeFramesMatchEveryTilesetAtlas(t *testing.T) {
	maxFrame := slices.Max(creepEdgeFrames[:])
	for folder := range blizzardTilesetDirs {
		atlas, err := CreepEdges(folder)
		if err != nil {
			t.Fatal(err)
		}
		if got := atlas.Bounds().Dx() / 32; got != maxFrame {
			t.Fatalf("%s: atlas has %d frames, table uses up to %d", folder, got, maxFrame)
		}
	}
}

func TestCreepEdgeFrame(t *testing.T) {
	if CreepEdgeFrame(0) != 0 {
		t.Fatal("a tile with no creep neighbors must not draw an edge")
	}
	if CreepEdgeFrame(0xff) == 0 {
		t.Fatal("a tile surrounded by creep must draw an edge")
	}
}

func TestCreepTexture(t *testing.T) {
	for folder := range blizzardTilesetDirs {
		img, err := CreepTexture(folder)
		if err != nil {
			t.Fatal(err)
		}
		if img.Bounds().Dx() != 128 || img.Bounds().Dy() != 128 {
			t.Fatalf("%s: got %v", folder, img.Bounds())
		}
	}
}
