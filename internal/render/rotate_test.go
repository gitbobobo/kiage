package render

import (
	"image"
	"image/color"
	"testing"
)

func TestShouldFlipPortraitPNG(t *testing.T) {
	if !shouldFlipPortraitPNG(0) {
		t.Fatal("rota=0 should flip")
	}
	if shouldFlipPortraitPNG(2) {
		t.Fatal("rota=2 should not flip")
	}
}

func TestRotateImage180(t *testing.T) {
	red := color.RGBA{R: 255, A: 255}
	blue := color.RGBA{B: 255, A: 255}
	src := image.NewRGBA(image.Rect(0, 0, 4, 2))
	src.Set(0, 0, red)
	src.Set(3, 1, blue)

	dst := rotateImage180(src)
	if c := dst.At(3, 1); c != red {
		t.Fatalf("top-left should move to bottom-right, got %v", c)
	}
	if c := dst.At(0, 0); c != blue {
		t.Fatalf("bottom-right should move to top-left, got %v", c)
	}
}
