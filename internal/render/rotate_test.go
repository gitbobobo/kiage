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

func TestPortraitRotaForDisplay(t *testing.T) {
	if got := PortraitRotaForDisplay(2, 0, 2); got != 2 {
		t.Fatalf("upright aligned: want 2 (no flip), got %d", got)
	}
	if got := PortraitRotaForDisplay(0, 0, 0); got != 2 {
		t.Fatalf("inverted aligned: want 2 (no flip), got %d", got)
	}
	if got := PortraitRotaForDisplay(0, 0, 2); got != 0 {
		t.Fatalf("2->0 rotated: want 0 (flip), got %d", got)
	}
	if got := PortraitRotaForDisplay(2, 0, 0); got != 0 {
		t.Fatalf("0->2 rotated: want 0 (flip), got %d", got)
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
