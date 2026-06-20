package render

// 固定区域高度（竖屏 Oasis 1072×1448）
const (
	PadX        = 28
	footerH     = 0
	topH        = 280
	heatH       = 152
	heatGapTop  = 12 // 热力图与上方折线图的间距
	topStatsBottomGap = 12 // 日月年统计与折线图间距
	topHPort        = 308
	topHPortKindle  = 368
	heatHPort   = 152
	heatDayLblW = 16
	heatMonthH      = 14
	heatBottomGap   = 4  // 网格与底行间距
	heatBottomTextH = 20 // 底行文字区域高度（含图例圆点）
)

func RegionHeights(orientation string) (top, heat int) {
	if orientation == "portrait" {
		if KindleUI() {
			return topHPortKindle, heatHPort
		}
		return topHPort, heatHPort
	}
	return topH, heatH
}

func regionHeights(orientation string) (top, heat int) {
	return RegionHeights(orientation)
}
