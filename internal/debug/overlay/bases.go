package overlay

import (
	"image"
	"image/color"
	"math"
	"strings"

	"github.com/marianogappa/scmapanalyzer/replaymap"
	xdraw "golang.org/x/image/draw"
)

// BaseShapes draws base masks (starts first, then expansions, matching result
// order) over the terrain at pxPerMinitile resolution, labelled by base name.
// With chokes set it also draws each base's choke segments.
func BaseShapes(terrain image.Image, result *replaymap.Result, masks [][]bool, w, h, pxPerMinitile int, chokes bool) *image.RGBA {
	canvas := image.NewRGBA(image.Rect(0, 0, w*pxPerMinitile, h*pxPerMinitile))
	xdraw.ApproxBiLinear.Scale(canvas, canvas.Bounds(), terrain, terrain.Bounds(), xdraw.Src, nil)
	step := float64(pxPerMinitile)

	bases := append(append([]replaymap.BasePolygon{}, result.Starts...), result.Expas...)
	for i, b := range bases {
		if i >= len(masks) {
			break
		}
		fill := color.RGBA{R: 255, G: 140, B: 0, A: 120}
		switch {
		case b.Kind == "start":
			fill = color.RGBA{R: 0, G: 230, B: 255, A: 120}
		case strings.HasPrefix(b.Name, "natural"):
			fill = color.RGBA{R: 150, G: 255, B: 0, A: 120}
		}
		drawMask(canvas, masks[i], w, h, step, step, fill, color.RGBA{R: 255, G: 255, B: 255, A: 230})
	}
	if chokes {
		for _, b := range bases {
			for _, c := range b.Chokes {
				drawChoke(canvas, c, step)
			}
		}
	}
	for i, b := range bases {
		if i >= len(masks) {
			break
		}
		cx, cy, ok := maskCentroid(masks[i], w, h)
		if !ok {
			continue
		}
		drawLabel(canvas, int(cx*step), int(cy*step), b.Name, max(1, pxPerMinitile))
	}
	return canvas
}

func drawChoke(canvas *image.RGBA, c replaymap.Choke, step float64) {
	ax, ay := (float64(c.Ends[0].X)+0.5)*step, (float64(c.Ends[0].Y)+0.5)*step
	bx, by := (float64(c.Ends[1].X)+0.5)*step, (float64(c.Ends[1].Y)+0.5)*step
	thick := step * 1.6
	drawThickLine(canvas, ax, ay, bx, by, thick+step, color.RGBA{A: 255})
	col := color.RGBA{R: 255, G: 235, B: 0, A: 255}
	if c.ConnectsTo == "" {
		col = color.RGBA{R: 255, G: 60, B: 200, A: 255}
	}
	drawThickLine(canvas, ax, ay, bx, by, thick, col)
}

func drawThickLine(canvas *image.RGBA, ax, ay, bx, by, thick float64, c color.RGBA) {
	minX := int(math.Floor(math.Min(ax, bx) - thick))
	maxX := int(math.Ceil(math.Max(ax, bx) + thick))
	minY := int(math.Floor(math.Min(ay, by) - thick))
	maxY := int(math.Ceil(math.Max(ay, by) + thick))
	dx, dy := bx-ax, by-ay
	l2 := dx*dx + dy*dy
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			px, py := float64(x)+0.5, float64(y)+0.5
			t := 0.0
			if l2 > 0 {
				t = math.Max(0, math.Min(1, ((px-ax)*dx+(py-ay)*dy)/l2))
			}
			if math.Hypot(px-ax-t*dx, py-ay-t*dy) <= thick/2 {
				blendRGBA(canvas, x, y, c)
			}
		}
	}
}

func maskCentroid(mask []bool, w, h int) (float64, float64, bool) {
	var sx, sy, n float64
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if mask[y*w+x] {
				sx += float64(x) + 0.5
				sy += float64(y) + 0.5
				n++
			}
		}
	}
	if n == 0 {
		return 0, 0, false
	}
	return sx / n, sy / n, true
}
