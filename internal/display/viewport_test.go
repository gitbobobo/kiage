package display

import "testing"

func TestTouchQuirkRota2(t *testing.T) {
	// Oasis 竖屏：fbink 报告 rota=2、面板 mx/my=false，须翻转为 mx/my=true
	vp := Viewport{CurrentRota: 2}
	q := vp.TouchQuirkForInput()
	if !q.MirrorX || !q.MirrorY {
		t.Fatalf("rota=2 got mx=%v my=%v want both true", q.MirrorX, q.MirrorY)
	}
}

func TestTouchQuirkForRota(t *testing.T) {
	vp := Viewport{TouchMirrorX: false, TouchMirrorY: false}
	q0 := vp.TouchQuirkForRota(0)
	if q0.MirrorX || q0.MirrorY {
		t.Fatalf("rota=0 got mx=%v my=%v", q0.MirrorX, q0.MirrorY)
	}
	q2 := vp.TouchQuirkForRota(2)
	if !q2.MirrorX || !q2.MirrorY {
		t.Fatalf("rota=2 got mx=%v my=%v want both true", q2.MirrorX, q2.MirrorY)
	}
}

func TestParseViewportEval(t *testing.T) {
	s := "FBINK_VERSION='1.25.0';viewWidth=1072;viewHeight=1448;screenWidth=1072;screenHeight=1448;touchSwapAxes=1;touchMirrorX=0;touchMirrorY=1;currentRota=0"
	vp := parseViewportEval(s)
	if vp.Width != 1072 || vp.Height != 1448 {
		t.Fatalf("got %dx%d", vp.Width, vp.Height)
	}
	if !vp.TouchSwapAxes || vp.TouchMirrorX || !vp.TouchMirrorY {
		t.Fatalf("touch flags swap=%v mx=%v my=%v", vp.TouchSwapAxes, vp.TouchMirrorX, vp.TouchMirrorY)
	}
	q := vp.TouchQuirkForInput()
	if !q.SwapAxes || !q.MirrorY {
		t.Fatalf("quirk swap=%v my=%v", q.SwapAxes, q.MirrorY)
	}
}
