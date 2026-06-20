package input

import "testing"

func TestTapSlop(t *testing.T) {
	if got := tapSlop(ScreenMapping{Width: 1264, Height: 1680}); got != 100 {
		t.Fatalf("kindle slop=%d want 100", got)
	}
	if got := tapSlop(ScreenMapping{Width: 800, Height: 600}); got != 50 {
		t.Fatalf("legacy slop=%d want 50", got)
	}
}

func TestMapTouchKindleViewport(t *testing.T) {
	screen := ScreenMapping{
		Width:  1264,
		Height: 1680,
		Quirk:  TouchQuirk{MirrorX: true, MirrorY: true},
	}
	bounds := TouchBounds{MaxX: 1071, MaxY: 1447}

	px, py := MapTouch(210, 1622, bounds, screen)
	if px < 1040 || px > 1070 {
		t.Fatalf("px=%d want ~1053", px)
	}
	if py < 40 || py > 60 {
		t.Fatalf("py=%d want ~45 without shift", py)
	}
}

func TestMapTouchSwapAxes(t *testing.T) {
	screen := ScreenMapping{Width: 1072, Height: 1448, Quirk: TouchQuirk{SwapAxes: true}}
	px, py := MapTouch(30, 1000, TouchBounds{MaxX: 1071, MaxY: 1447}, screen)
	if px != 1000 || py != 30 {
		t.Fatalf("swap got (%d,%d) want (1000,30)", px, py)
	}
}

func TestMapTouchMirrorX(t *testing.T) {
	screen := ScreenMapping{Width: 1072, Height: 1448, Quirk: TouchQuirk{MirrorX: true}}
	px, py := MapTouch(1000, 30, TouchBounds{MaxX: 1071, MaxY: 1447}, screen)
	if px != 71 || py != 30 {
		t.Fatalf("mirrorX got (%d,%d) want (71,30)", px, py)
	}
}
