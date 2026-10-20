package mapgfx

import (
	"image"
	"image/draw"
	"testing"
)

func TestTerrainAndResourceLayersRebuildTheCombinedCrop(t *testing.T) {
	md := MapData{TileSet: "badlands", Width: 8, Height: 8, Tiles: make([]uint16, 64), Minerals: []Point{{X: 64, Y: 80}}, Geysers: []Point{{X: 190, Y: 200}}}
	for i := range md.Tiles {
		md.Tiles[i] = uint16(32 + i%9)
	}
	r := TileRect{X: 1, Y: 1, W: 6, H: 6}
	render := func(opts RenderOptions) image.Image {
		img, err := renderMapRegion(md, opts, r)
		if err != nil {
			t.Fatal(err)
		}
		return img
	}
	combined := render(RenderOptions{OverlayResources: true})
	terrain := render(RenderOptions{})
	resources := render(RenderOptions{ResourcesOnly: true}).(*image.RGBA)
	if resources.Bounds() != combined.Bounds() || terrain.Bounds() != combined.Bounds() {
		t.Fatalf("bounds differ: %v %v %v", terrain.Bounds(), resources.Bounds(), combined.Bounds())
	}
	if resources.RGBAAt(r.X*32, r.Y*32).A != 0 {
		t.Fatal("expected transparency away from resources")
	}
	rebuilt := image.NewRGBA(combined.Bounds())
	draw.Draw(rebuilt, rebuilt.Bounds(), terrain, rebuilt.Bounds().Min, draw.Src)
	draw.Draw(rebuilt, rebuilt.Bounds(), resources, rebuilt.Bounds().Min, draw.Over)
	for y := rebuilt.Bounds().Min.Y; y < rebuilt.Bounds().Max.Y; y++ {
		for x := rebuilt.Bounds().Min.X; x < rebuilt.Bounds().Max.X; x++ {
			if rebuilt.At(x, y) != combined.At(x, y) {
				t.Fatalf("pixel (%d,%d) differs", x, y)
			}
		}
	}

	shadowed := render(RenderOptions{ResourcesOnly: true, ResourceShadows: true}).(*image.RGBA)
	more := 0
	for i := 3; i < len(shadowed.Pix); i += 4 {
		if shadowed.Pix[i] != 0 && resources.Pix[i] == 0 {
			more++
		}
	}
	if more == 0 {
		t.Fatal("expected shadows around the resources")
	}
}
