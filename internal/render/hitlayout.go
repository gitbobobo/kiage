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
}

const topTitleY = 16

// TopControlsHitRegions 返回顶部可点击区域。
func TopControlsHitRegions(screen Screen, providerName string) TopHitRegions {
	if screen == ScreenSummary {
		return TopHitRegions{}
	}
	titleW := textWidth(providerName, TitleFontSize())
	if titleW < 48 {
		titleW = 48
	}

	return TopHitRegions{
		ProviderTitle: HitRect{
			X: PadX,
			Y: topTitleY,
			W: titleW + 16,
			H: TopTitleHitHeight(),
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
