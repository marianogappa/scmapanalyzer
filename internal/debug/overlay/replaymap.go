package overlay

import (
	"image"
	"image/color"
	"image/draw"

	"github.com/marianogappa/scmapanalyzer/replaymap"
)

var expaPalette = []struct {
	fill   color.RGBA
	stroke color.RGBA
}{
	{fill: color.RGBA{R: 255, G: 60, B: 0, A: 175}, stroke: color.RGBA{R: 255, G: 255, B: 255, A: 255}},
	{fill: color.RGBA{R: 255, G: 180, B: 0, A: 175}, stroke: color.RGBA{R: 255, G: 255, B: 255, A: 255}},
	{fill: color.RGBA{R: 160, G: 255, B: 0, A: 175}, stroke: color.RGBA{R: 255, G: 255, B: 255, A: 255}},
	{fill: color.RGBA{R: 0, G: 200, B: 255, A: 175}, stroke: color.RGBA{R: 255, G: 255, B: 255, A: 255}},
	{fill: color.RGBA{R: 180, G: 0, B: 255, A: 175}, stroke: color.RGBA{R: 255, G: 255, B: 255, A: 255}},
	{fill: color.RGBA{R: 255, G: 0, B: 180, A: 175}, stroke: color.RGBA{R: 255, G: 255, B: 255, A: 255}},
}

func ReplayMap(base image.Image, debug *replaymap.DebugData) image.Image {
	bounds := base.Bounds()
	canvas := image.NewRGBA(bounds)
	draw.Draw(canvas, bounds, base, bounds.Min, draw.Src)
	if debug == nil {
		return canvas
	}
	stepX := float64(bounds.Dx()) / float64(debug.WidthTiles)
	stepY := float64(bounds.Dy()) / float64(debug.HeightTiles)

	startFill := color.RGBA{R: 0, G: 255, B: 255, A: 190}
	startStroke := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	for _, mask := range debug.StartMasks {
		drawMask(canvas, mask, debug.WidthTiles, debug.HeightTiles, stepX, stepY, startFill, startStroke)
	}
	for i, mask := range debug.ExpaMasks {
		p := expaPalette[i%len(expaPalette)]
		drawMask(canvas, mask, debug.WidthTiles, debug.HeightTiles, stepX, stepY, p.fill, p.stroke)
	}
	for i, l := range debug.NaturalLinks {
		p := expaPalette[i%len(expaPalette)].stroke
		for _, tp := range l.Path {
			rect := tileRect(tp.X, tp.Y, stepX, stepY, bounds)
			fillRect(canvas, rect, color.RGBA{R: p.R, G: p.G, B: p.B, A: 140})
		}
	}
	wallStroke := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	rampStroke := color.RGBA{R: 160, G: 0, B: 255, A: 255}
	drawCategoryTileOutlines(canvas, debug.WallMask, debug.WidthTiles, debug.HeightTiles, stepX, stepY, wallStroke)
	drawCategoryTileOutlines(canvas, debug.RampMask, debug.WidthTiles, debug.HeightTiles, stepX, stepY, rampStroke)
	return canvas
}

func drawCategoryTileOutlines(canvas *image.RGBA, mask []bool, width int, height int, stepX float64, stepY float64, stroke color.RGBA) {
	if len(mask) == 0 || width <= 0 || height <= 0 {
		return
	}
	b := canvas.Bounds()
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x
			if !mask[idx] {
				continue
			}
			rect := tileRect(x, y, stepX, stepY, b)
			if x == 0 || !mask[idx-1] {
				drawVertical(canvas, rect.Min.X, rect.Min.Y, rect.Max.Y, stroke)
			}
			if x == width-1 || !mask[idx+1] {
				drawVertical(canvas, rect.Max.X-1, rect.Min.Y, rect.Max.Y, stroke)
			}
			if y == 0 || !mask[idx-width] {
				drawHorizontal(canvas, rect.Min.X, rect.Max.X, rect.Min.Y, stroke)
			}
			if y == height-1 || !mask[idx+width] {
				drawHorizontal(canvas, rect.Min.X, rect.Max.X, rect.Max.Y-1, stroke)
			}
		}
	}
}

func drawMask(canvas *image.RGBA, mask []bool, width int, height int, stepX float64, stepY float64, fill color.RGBA, stroke color.RGBA) {
	if mask == nil {
		return
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x
			if !mask[idx] {
				continue
			}
			rect := tileRect(x, y, stepX, stepY, canvas.Bounds())
			fillRect(canvas, rect, fill)
			if x == 0 || !mask[idx-1] {
				drawVertical(canvas, rect.Min.X, rect.Min.Y, rect.Max.Y, stroke)
			}
			if x == width-1 || !mask[idx+1] {
				drawVertical(canvas, rect.Max.X-1, rect.Min.Y, rect.Max.Y, stroke)
			}
			if y == 0 || !mask[idx-width] {
				drawHorizontal(canvas, rect.Min.X, rect.Max.X, rect.Min.Y, stroke)
			}
			if y == height-1 || !mask[idx+width] {
				drawHorizontal(canvas, rect.Min.X, rect.Max.X, rect.Max.Y-1, stroke)
			}
		}
	}
}

func tileRect(x int, y int, stepX float64, stepY float64, bounds image.Rectangle) image.Rectangle {
	x0 := bounds.Min.X + int(stepX*float64(x)+0.5)
	x1 := bounds.Min.X + int(stepX*float64(x+1)+0.5)
	y0 := bounds.Min.Y + int(stepY*float64(y)+0.5)
	y1 := bounds.Min.Y + int(stepY*float64(y+1)+0.5)
	if x1 <= x0 {
		x1 = x0 + 1
	}
	if y1 <= y0 {
		y1 = y0 + 1
	}
	return image.Rect(x0, y0, x1, y1).Intersect(bounds)
}

func fillRect(img *image.RGBA, rect image.Rectangle, c color.RGBA) {
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			blendRGBA(img, x, y, c)
		}
	}
}

func drawVertical(img *image.RGBA, x int, y0 int, y1 int, c color.RGBA) {
	for y := y0; y < y1; y++ {
		blendRGBA(img, x, y, c)
	}
}

func drawHorizontal(img *image.RGBA, x0 int, x1 int, y int, c color.RGBA) {
	for x := x0; x < x1; x++ {
		blendRGBA(img, x, y, c)
	}
}

func blendRGBA(img *image.RGBA, x int, y int, src color.RGBA) {
	if !image.Pt(x, y).In(img.Bounds()) {
		return
	}
	dst := img.RGBAAt(x, y)
	a := float64(src.A) / 255.0
	ia := 1.0 - a
	out := color.RGBA{
		R: uint8(float64(src.R)*a + float64(dst.R)*ia),
		G: uint8(float64(src.G)*a + float64(dst.G)*ia),
		B: uint8(float64(src.B)*a + float64(dst.B)*ia),
		A: 255,
	}
	img.SetRGBA(x, y, out)
}
