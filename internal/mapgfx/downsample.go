package mapgfx

import (
	"image"
	"image/draw"
)

// downsampleFactor picks the smallest power-of-two reduction whose result fits
// under maxDim on the longest side. Powers of two keep the box average a clean
// mipmap step for pixel art, which stays crisp where resampling kernels are soft.
func downsampleFactor(w, h, maxDim int) int {
	longest := max(w, h)
	factor := 1
	for longest/factor > maxDim {
		factor *= 2
	}
	return factor
}

func boxDownsample(img image.Image, factor int) *image.RGBA {
	src, ok := img.(*image.RGBA)
	if !ok || src.Bounds().Min != (image.Point{}) {
		src = image.NewRGBA(image.Rect(0, 0, img.Bounds().Dx(), img.Bounds().Dy()))
		draw.Draw(src, src.Bounds(), img, img.Bounds().Min, draw.Src)
	}
	srcW, srcH := src.Bounds().Dx(), src.Bounds().Dy()
	dstW, dstH := srcW/factor, srcH/factor
	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	area := uint32(factor * factor)
	for dy := 0; dy < dstH; dy++ {
		for dx := 0; dx < dstW; dx++ {
			var r, g, b, a uint32
			for sy := dy * factor; sy < (dy+1)*factor; sy++ {
				rowOff := sy*src.Stride + dx*factor*4
				for sx := 0; sx < factor; sx++ {
					off := rowOff + sx*4
					r += uint32(src.Pix[off])
					g += uint32(src.Pix[off+1])
					b += uint32(src.Pix[off+2])
					a += uint32(src.Pix[off+3])
				}
			}
			off := dy*dst.Stride + dx*4
			dst.Pix[off] = uint8(r / area)
			dst.Pix[off+1] = uint8(g / area)
			dst.Pix[off+2] = uint8(b / area)
			dst.Pix[off+3] = uint8(a / area)
		}
	}
	return dst
}
