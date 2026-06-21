package render

import "image"

// rotateImage180 将 RGBA 图像旋转 180°。
func rotateImage180(src *image.RGBA) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(b)
	w := b.Dx()
	h := b.Dy()
	if w == 0 || h == 0 {
		return dst
	}
	srcStride := src.Stride
	dstStride := dst.Stride
	for y := 0; y < h; y++ {
		srcRow := y * srcStride
		dstRow := (h - 1 - y) * dstStride
		for x := 0; x < w; x++ {
			srcOff := srcRow + x*4
			dstOff := dstRow + (w-1-x)*4
			dst.Pix[dstOff] = src.Pix[srcOff]
			dst.Pix[dstOff+1] = src.Pix[srcOff+1]
			dst.Pix[dstOff+2] = src.Pix[srcOff+2]
			dst.Pix[dstOff+3] = src.Pix[srcOff+3]
		}
	}
	return dst
}

// shouldFlipPortraitPNG 在 FBINK_NO_SW_ROTA=1 下，仅 rota=0 时翻转 PNG。
func shouldFlipPortraitPNG(portraitRota int) bool {
	return portraitRota == 0
}
