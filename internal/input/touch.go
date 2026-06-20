package input

import "os"

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

	// 仅当坐标落在 ABS 范围内且与视口尺寸不同时才缩放
	if tx >= 0 && tx <= maxX && maxX > 0 && maxX+1 != w {
		x = tx * (w - 1) / maxX
	}
	if ty >= 0 && ty <= maxY && maxY > 0 && maxY+1 != h {
		y = ty * (h - 1) / maxY
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
	return x, y
}

func screenPortraitFromEnv() bool {
	return os.Getenv("KIAGE_PORTRAIT") == "1"
}
