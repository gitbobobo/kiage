package input

import "testing"

func TestMapTouchKindleViewport(t *testing.T) {
	// 来自真机日志：raw=(210,1622) screen=1264x1680 max=1071x1447 mx/my=true rota=2
	screen := ScreenMapping{
		Width:  1264,
		Height: 1680,
		Quirk:  TouchQuirk{MirrorX: true, MirrorY: true},
	}
	bounds := TouchBounds{MaxX: 1071, MaxY: 1447}
	px, py := MapTouch(210, 1622, bounds, screen)
	if px < 950 || px > 1050 {
		t.Fatalf("px=%d want ~1016", px)
	}
	if py < 0 || py > 120 {
		t.Fatalf("py=%d want ~57", py)
	}

	// 16:36 会话：无旋转修正时 mapped=(234,1643) 在底部；修正后应落在顶部
	px, py = MapTouch(199, 1643, bounds, screen)
	if py < 0 || py > 120 {
		t.Fatalf("1643 tap py=%d want top ~36", py)
	}
	if px < 950 {
		t.Fatalf("1643 tap px=%d want right side", px)
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
