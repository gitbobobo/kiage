package display

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Viewport 为 FBInk -e 报告的可绘制区域与触摸变换参数。
type Viewport struct {
	Width  int
	Height int
	// 触摸坐标变换（与 FBInk finger_trace 一致，已含机型面板特性）
	TouchSwapAxes bool
	TouchMirrorX  bool
	TouchMirrorY  bool
	CurrentRota   int
}

// TouchQuirk 供 input 包做坐标映射。
type TouchQuirk struct {
	SwapAxes bool
	MirrorX  bool
	MirrorY  bool
}

// TouchQuirkForInput 按当前旋转调整触摸变换标志（与 FBInk finger_trace 一致）。
// fbink -e 的 touchMirror* 为面板原生方向；currentRota 表示相对该方向的视口旋转，须叠加修正。
func (vp Viewport) TouchQuirkForInput() TouchQuirk {
	q := TouchQuirk{
		SwapAxes: vp.TouchSwapAxes,
		MirrorX:  vp.TouchMirrorX,
		MirrorY:  vp.TouchMirrorY,
	}
	return applyRotationQuirk(q, vp.CurrentRota)
}

// applyRotationQuirk 与 FBInk finger_trace 的 canonical rotation 处理一致。
func applyRotationQuirk(q TouchQuirk, rota int) TouchQuirk {
	switch rota {
	case 1: // FB_ROTATE_CW
		q.SwapAxes = !q.SwapAxes
		q.MirrorY = !q.MirrorY
	case 2: // FB_ROTATE_UD
		q.MirrorX = !q.MirrorX
		q.MirrorY = !q.MirrorY
	case 3: // FB_ROTATE_CCW
		q.SwapAxes = !q.SwapAxes
		q.MirrorX = !q.MirrorX
	}
	return q
}

// QueryViewport 调用 fbink -e 获取当前视口尺寸与触摸参数。
func QueryViewport(bin string) (Viewport, error) {
	if bin == "" {
		bin = ResolveFBInkBin()
	}
	out, err := exec.Command(bin, "-e").Output()
	if err != nil {
		return Viewport{}, fmt.Errorf("fbink -e: %w", err)
	}
	vp := parseViewportEval(string(out))
	if vp.Width <= 0 || vp.Height <= 0 {
		return Viewport{}, fmt.Errorf("fbink -e: invalid viewport from %q", strings.TrimSpace(string(out)))
	}
	return vp, nil
}

func parseViewportEval(s string) Viewport {
	var vp Viewport
	for _, part := range strings.Split(s, ";") {
		part = strings.TrimSpace(part)
		if w, ok := parseEvalUint(part, "viewWidth"); ok {
			vp.Width = w
		}
		if h, ok := parseEvalUint(part, "viewHeight"); ok {
			vp.Height = h
		}
		if v, ok := parseEvalBool(part, "touchSwapAxes"); ok {
			vp.TouchSwapAxes = v
		}
		if v, ok := parseEvalBool(part, "touchMirrorX"); ok {
			vp.TouchMirrorX = v
		}
		if v, ok := parseEvalBool(part, "touchMirrorY"); ok {
			vp.TouchMirrorY = v
		}
		if v, ok := parseEvalUint(part, "currentRota"); ok {
			vp.CurrentRota = v
		}
	}
	return vp
}

func parseEvalUint(part, key string) (int, bool) {
	prefix := key + "="
	if !strings.HasPrefix(part, prefix) {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(part, prefix))
	if err != nil {
		return 0, false
	}
	return n, true
}

func parseEvalBool(part, key string) (bool, bool) {
	prefix := key + "="
	if !strings.HasPrefix(part, prefix) {
		return false, false
	}
	v := strings.TrimPrefix(part, prefix)
	return v == "1", true
}
