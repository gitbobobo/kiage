package input

import (
	"os"
	"strconv"
)

// TouchQuirk 触摸屏坐标变换（来自 FBInk -e 的 touchSwapAxes/Mirror 标志）。
type TouchQuirk struct {
	SwapAxes bool
	MirrorX  bool
	MirrorY  bool
}

// ScreenMapping 触摸坐标到 PNG 像素的映射参数。
type ScreenMapping struct {
	Width  int
	Height int
	Quirk  TouchQuirk
}

// TouchBounds 触摸屏 ABS 轴范围（来自 EVIOCGABS）。
type TouchBounds struct {
	MaxX int
	MaxY int
}

// Handler 处理短按。
type Handler interface {
	OnTap(x, y int)
}

type QuirkVersionHandler interface {
	Handler
	TouchQuirkVersion() uint64
}

// MapTouch 将设备触摸坐标映射为 PNG 像素坐标。
// Kindle 上触摸事件常在视口坐标系（1264×1680）内，与 ABS max（1072×1448）不一致。
func MapTouch(tx, ty int, bounds TouchBounds, screen ScreenMapping) (int, int) {
	w := screen.Width
	h := screen.Height
	if w <= 0 {
		w = 1072
	}
	if h <= 0 {
		h = 1448
	}

	maxX := bounds.MaxX
	maxY := bounds.MaxY
	if maxX <= 0 {
		maxX = w - 1
	}
	if maxY <= 0 {
		maxY = h - 1
	}

	x, y := tx, ty

	// 竖屏时 ty 常超出 ABS maxY，此时 X/Y 均在视口坐标系内，勿做 ABS 缩放
	viewportCoords := ty > maxY || tx > maxX
	if !viewportCoords {
		if tx >= 0 && tx <= maxX && maxX > 0 && maxX+1 != w {
			x = tx * (w - 1) / maxX
		}
		if ty >= 0 && ty <= maxY && maxY > 0 && maxY+1 != h {
			y = ty * (h - 1) / maxY
		}
	}

	q := screen.Quirk
	if q.SwapAxes {
		x, y = y, x
	}
	if q.MirrorX {
		x = w - 1 - x
	}
	if q.MirrorY {
		y = h - 1 - y
	}

	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x >= w {
		x = w - 1
	}
	if y >= h {
		y = h - 1
	}

	// 可选微调：正数=映射点下移，负数=上移（默认 0，由日志实测再调）
	if screenPortraitFromEnv() {
		y += touchYShift()
		if y < 0 {
			y = 0
		}
		if y >= h {
			y = h - 1
		}
	}
	return x, y
}

func touchYShift() int {
	if v := os.Getenv("KIAGE_TOUCH_Y_SHIFT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return 0
}

func screenPortraitFromEnv() bool {
	return os.Getenv("KIAGE_PORTRAIT") == "1"
}

// tapSlop 允许的单次点击最大位移（像素）。
func tapSlop(screen ScreenMapping) int {
	if screen.Height >= 1400 || screen.Width >= 1400 {
		return 100
	}
	return 50
}
