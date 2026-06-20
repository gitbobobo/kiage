package render

// HitRect 可点击区域（PNG 像素坐标）
type HitRect struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

type TopHitRegions struct {
	ProviderTitle HitRect `json:"provider_title"`
	Exit          HitRect `json:"exit"`
	Settings      HitRect `json:"settings"`
	MetricToggle  HitRect `json:"metric_toggle"`
}

const topTitleY = 16

const topControlsY = 18 // drawTopSection y=16 + controls y+2

// TopControlsHitRegions 返回顶部控件的可点击区域
func TopControlsHitRegions(size Size, providerName string) TopHitRegions {
	w := size.Width - PadX*2
	rightX := PadX + w
	btnSz := SettingsBtnSize()
	gap := ControlsGap()
	exitX := rightX - btnSz
	settingsX := exitX - gap - btnSz
	toggleW := metricToggleWidth("token")
	toggleX := settingsX - gap - toggleW

	titleW := textWidth(providerName, TitleFontSize())
	if titleW < 48 {
		titleW = 48
	}

	controlsY := topControlsY

	return TopHitRegions{
		ProviderTitle: HitRect{
			X: PadX,
			Y: topTitleY,
			W: titleW + 16,
			H: TopTitleHitHeight(),
		},
		Exit: HitRect{
			X: exitX,
			Y: controlsY,
			W: btnSz,
			H: btnSz,
		},
		Settings: HitRect{
			X: settingsX,
			Y: controlsY,
			W: btnSz,
			H: btnSz,
		},
		MetricToggle: HitRect{
			X: toggleX,
			Y: controlsY,
			W: toggleW,
			H: MetricToggleHeight(),
		},
	}
}

func (r HitRect) Contains(x, y int) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

// HitTopRightBar 按屏幕比例划分右上角触控区（比像素级热区更耐坐标偏差）。
// 从右到左：退出 | 设置 | Token/Cost
func HitTopRightBar(size Size, x, y int) string {
	const barH = 64
	if y < topControlsY || y >= topControlsY+barH {
		return ""
	}
	left := size.Width * 48 / 100
	if x < left {
		return ""
	}
	span := size.Width - left
	rel := x - left
	third := span / 3
	switch {
	case rel >= 2*third:
		return "exit"
	case rel >= third:
		return "settings"
	default:
		return "metric_toggle"
	}
}
