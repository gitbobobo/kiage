package render

// HitRect 可点击区域（PNG 像素坐标）
type HitRect struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

type TopHitRegions struct {
	ProviderTitle  HitRect `json:"provider_title"`
	ProviderToggle HitRect `json:"provider_toggle"`
	Exit           HitRect `json:"exit"`
	Settings       HitRect `json:"settings"`
	MetricToggle   HitRect `json:"metric_toggle"`
}

const topTitleY = 16

// TopControlsHitRegions 返回顶部控件的可点击区域
func TopControlsHitRegions(size Size, providerName, chartMetric, providerID string) TopHitRegions {
	w := size.Width - PadX*2
	rightX := PadX + w
	btnSz := SettingsBtnSize()
	gap := ControlsGap()
	exitX := rightX - btnSz
	settingsX := exitX - gap - btnSz
	if chartMetric == "" {
		chartMetric = "token"
	}
	metricW := metricToggleWidth(chartMetric)
	metricX := settingsX - gap - metricW
	providerW := providerToggleWidth()
	providerX := metricX - gap - providerW

	titleW := textWidth(providerName, TitleFontSize())
	if titleW < 48 {
		titleW = 48
	}

	controlsY := TopControlsY() + 2

	return TopHitRegions{
		ProviderTitle: HitRect{
			X: PadX,
			Y: topTitleY,
			W: titleW + 16,
			H: TopTitleHitHeight(),
		},
		ProviderToggle: HitRect{
			X: providerX,
			Y: controlsY,
			W: providerW,
			H: MetricToggleHeight(),
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
			X: metricX,
			Y: controlsY,
			W: metricW,
			H: MetricToggleHeight(),
		},
	}
}

func (r HitRect) Contains(x, y int) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

func (r HitRect) ContainsPad(x, y, pad int) bool {
	return r.ContainsPadAsymmetric(x, y, pad, pad, pad, pad)
}

func (r HitRect) ContainsPadAsymmetric(x, y, padL, padT, padR, padB int) bool {
	return x >= r.X-padL && x < r.X+r.W+padR &&
		y >= r.Y-padT && y < r.Y+r.H+padB
}

// HitTopRightBar 按屏幕比例划分右上角触控区（网页预览兜底）。
// 从右到左：退出 | 设置 | Token/Cost | Cursor/GLM
func HitTopRightBar(size Size, x, y int) string {
	barH := 64
	if KindleUI() {
		barH = 72
	}
	controlsY := TopControlsY() + 2
	if y < controlsY || y >= controlsY+barH {
		return ""
	}
	left := size.Width * 42 / 100
	if !KindleUI() {
		left = size.Width * 45 / 100
	}
	if x < left {
		return ""
	}
	span := size.Width - left
	rel := x - left
	quarter := span / 4
	switch {
	case rel >= 3*quarter:
		return "exit"
	case rel >= 2*quarter:
		return "settings"
	case rel >= quarter:
		return "metric_toggle"
	default:
		return "provider_toggle"
	}
}
