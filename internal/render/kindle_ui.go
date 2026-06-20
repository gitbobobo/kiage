package render

import (
	"os"
	"runtime"
)

// KindleUI 为 true 时使用更大字号与触控热区（Kindle 竖屏）。
func KindleUI() bool {
	if os.Getenv("KIAGE_KINDLE_UI") == "1" || os.Getenv("KIAGE_PORTRAIT") == "1" {
		return true
	}
	return runtime.GOOS == "linux"
}

func TitleFontSize() int {
	if KindleUI() {
		return 34
	}
	return 26
}

func StatusFontSize() int {
	if KindleUI() {
		return 18
	}
	return 14
}

func PlanFontSize() int {
	if KindleUI() {
		return 20
	}
	return 16
}

func SettingsBtnSize() int {
	if KindleUI() {
		return 48
	}
	return settingsBtnSize
}

func SettingsIconSize() int {
	if KindleUI() {
		return 28
	}
	return settingsIconSz
}

func ControlsGap() int {
	if KindleUI() {
		return 12
	}
	return controlsGap
}

func MetricToggleHeight() int {
	if KindleUI() {
		return 44
	}
	return 24
}

func MetricToggleFontSize() int {
	if KindleUI() {
		return 19
	}
	return 12
}

func MetricTogglePadX() int {
	if KindleUI() {
		return 16
	}
	return 10
}

func PeriodBoxHeight() int {
	if KindleUI() {
		return 82
	}
	return 58
}

func PeriodTitleFontSize() int {
	if KindleUI() {
		return 22
	}
	return 15
}

func PeriodValueFontSize() int {
	if KindleUI() {
		return 30
	}
	return 15
}

func TopTitleHitHeight() int {
	if KindleUI() {
		return 44
	}
	return 34
}

func TopControlsY() int {
	if KindleUI() {
		return 16
	}
	return 18
}
