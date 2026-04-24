package overlay

import (
	"image"
	"image/color"
	"image/draw"
	"math"

	"github.com/marianogappa/scmapanalyzer/replaymap"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
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

func ReplayMap(base image.Image, result *replaymap.Result, debug *replaymap.DebugData) image.Image {
	bounds := base.Bounds()
	canvas := image.NewRGBA(bounds)
	draw.Draw(canvas, bounds, base, bounds.Min, draw.Src)
	if debug == nil {
		return canvas
	}
	stepX := float64(bounds.Dx()) / float64(debug.WidthMinitiles)
	stepY := float64(bounds.Dy()) / float64(debug.HeightMinitiles)

	startFill := color.RGBA{R: 0, G: 255, B: 255, A: 190}
	startStroke := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	for _, mask := range debug.StartMasks {
		drawMask(canvas, mask, debug.WidthMinitiles, debug.HeightMinitiles, stepX, stepY, startFill, startStroke)
	}
	for i, mask := range debug.ExpaMasks {
		p := expaPalette[i%len(expaPalette)]
		drawMask(canvas, mask, debug.WidthMinitiles, debug.HeightMinitiles, stepX, stepY, p.fill, p.stroke)
	}
	for i, l := range debug.NaturalLinks {
		p := expaPalette[i%len(expaPalette)].stroke
		for _, tp := range l.Path {
			rect := tileRect(tp.X, tp.Y, stepX, stepY, bounds)
			fillRect(canvas, rect, color.RGBA{R: p.R, G: p.G, B: p.B, A: 140})
		}
	}
	wallFill := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	wallStroke := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	rampFill := color.RGBA{R: 160, G: 0, B: 255, A: 255}
	rampStroke := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	drawCategoryTiles(canvas, debug.WallMask, debug.WidthMinitiles, debug.HeightMinitiles, stepX, stepY, wallFill, wallStroke)
	drawCategoryTiles(canvas, debug.RampMask, debug.WidthMinitiles, debug.HeightMinitiles, stepX, stepY, rampFill, rampStroke)
	if result != nil {
		for _, b := range result.Starts {
			drawBaseLabel(canvas, b, stepX, stepY, bounds)
		}
		for _, b := range result.Expas {
			drawBaseLabel(canvas, b, stepX, stepY, bounds)
		}
	}
	return canvas
}

func drawBaseLabel(canvas *image.RGBA, b replaymap.BasePolygon, stepX float64, stepY float64, bounds image.Rectangle) {
	if b.Name == "" {
		return
	}
	polyCxMini, polyCyMini := polygonCenter(b.PolygonVertices)
	if math.IsNaN(polyCxMini) {
		polyCxMini = float64(b.CenterTile.X) + 0.5
		polyCyMini = float64(b.CenterTile.Y) + 0.5
	}
	cx := bounds.Min.X + int(stepX*polyCxMini+0.5)
	cy := bounds.Min.Y + int(stepY*polyCyMini+0.5)
	scale := labelScaleForPolygon(b.PolygonVertices, stepX, b.Name)
	drawLabel(canvas, cx, cy, b.Name, scale)
}

func polygonCenter(vertices []replaymap.TilePoint) (float64, float64) {
	n := len(vertices)
	if n == 0 {
		return math.NaN(), math.NaN()
	}
	if n < 3 {
		sx, sy := 0.0, 0.0
		for _, v := range vertices {
			sx += float64(v.X)
			sy += float64(v.Y)
		}
		return sx / float64(n), sy / float64(n)
	}
	var area, cx, cy float64
	for i := 0; i < n; i++ {
		x0 := float64(vertices[i].X)
		y0 := float64(vertices[i].Y)
		x1 := float64(vertices[(i+1)%n].X)
		y1 := float64(vertices[(i+1)%n].Y)
		cross := x0*y1 - x1*y0
		area += cross
		cx += (x0 + x1) * cross
		cy += (y0 + y1) * cross
	}
	area *= 0.5
	if area == 0 {
		minX, maxX := float64(vertices[0].X), float64(vertices[0].X)
		minY, maxY := float64(vertices[0].Y), float64(vertices[0].Y)
		for _, v := range vertices {
			x, y := float64(v.X), float64(v.Y)
			if x < minX {
				minX = x
			}
			if x > maxX {
				maxX = x
			}
			if y < minY {
				minY = y
			}
			if y > maxY {
				maxY = y
			}
		}
		return (minX + maxX) / 2, (minY + maxY) / 2
	}
	return cx / (6 * area), cy / (6 * area)
}

func labelScaleForPolygon(vertices []replaymap.TilePoint, stepX float64, text string) int {
	const fallback = 5
	if len(vertices) == 0 || text == "" {
		return fallback
	}
	minX, maxX := vertices[0].X, vertices[0].X
	for _, v := range vertices {
		if v.X < minX {
			minX = v.X
		}
		if v.X > maxX {
			maxX = v.X
		}
	}
	polyWidthPx := float64(maxX-minX) * stepX
	if polyWidthPx <= 0 {
		return fallback
	}
	textWidth := float64(font.MeasureString(basicfont.Face7x13, text).Round())
	if textWidth <= 0 {
		return fallback
	}
	scale := int(polyWidthPx * 0.85 / textWidth)
	if scale < 2 {
		scale = 2
	}
	if scale > 10 {
		scale = 10
	}
	return scale
}

func drawLabel(canvas *image.RGBA, cx int, cy int, text string, scale int) {
	if scale < 1 {
		scale = 1
	}
	face := basicfont.Face7x13
	textWidth := font.MeasureString(face, text).Round()
	ascent := face.Metrics().Ascent.Round()
	descent := face.Metrics().Descent.Round()
	height := ascent + descent
	if textWidth <= 0 || height <= 0 {
		return
	}
	small := image.NewRGBA(image.Rect(0, 0, textWidth, height))
	d := &font.Drawer{
		Dst:  small,
		Src:  image.NewUniform(color.RGBA{R: 255, G: 255, B: 255, A: 255}),
		Face: face,
		Dot:  fixed.P(0, ascent),
	}
	d.DrawString(text)
	sw := textWidth * scale
	sh := height * scale
	big := image.NewRGBA(image.Rect(0, 0, sw, sh))
	for y := 0; y < sh; y++ {
		sy := y / scale
		for x := 0; x < sw; x++ {
			big.SetRGBA(x, y, small.RGBAAt(x/scale, sy))
		}
	}
	topX := cx - sw/2
	topY := cy - sh/2
	halo := scale / 2
	if halo < 1 {
		halo = 1
	}
	black := color.RGBA{R: 0, G: 0, B: 0, A: 255}
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	for dy := -halo; dy <= halo; dy++ {
		for dx := -halo; dx <= halo; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			blitMask(canvas, big, topX+dx, topY+dy, black)
		}
	}
	blitMask(canvas, big, topX, topY, white)
}

func blitMask(dst *image.RGBA, src *image.RGBA, ox int, oy int, c color.RGBA) {
	b := src.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if src.RGBAAt(x, y).A == 0 {
				continue
			}
			blendRGBA(dst, ox+x, oy+y, c)
		}
	}
}

func drawCategoryTiles(canvas *image.RGBA, mask []bool, width int, height int, stepX float64, stepY float64, fill color.RGBA, stroke color.RGBA) {
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
