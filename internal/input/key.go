package input

import (
	"os"
	"strings"
)

const (
	KeyUp       = 103
	KeyDown     = 108
	KeyPageUp   = 104
	KeyPageDown = 109
)

// KeyHandler 处理屏幕「上」方向键单击。
type KeyHandler interface {
	OnScreenUp()
	PortraitRota() int
}

// ScreenUpKey 判断按键是否为当前屏幕朝向下的「上」方向键（portraitRota 0/2）。
func ScreenUpKey(code, portraitRota int) bool {
	if portraitRota != 2 {
		portraitRota = 0
	}
	switch code {
	case KeyUp, KeyDown:
		if portraitRota == 2 {
			return code == KeyDown
		}
		return code == KeyUp
	case KeyPageUp, KeyPageDown:
		upAt0 := pageUpAtRota0()
		if portraitRota == 2 {
			return code == otherPageKey(upAt0)
		}
		return code == upAt0
	default:
		return false
	}
}

func otherPageKey(k int) int {
	if k == KeyPageUp {
		return KeyPageDown
	}
	return KeyPageUp
}

func pageUpAtRota0() int {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("KIAGE_SCREEN_UP_KEY"))) {
	case "pagedown", "page_down":
		return KeyPageDown
	default:
		return KeyPageUp
	}
}
