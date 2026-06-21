package render

// KindleControlBarYMax 顶部控件行在 Y 轴上的可接受范围（仅校验 X 列）。
const KindleControlBarYMax = 120

// KindleTopControlAction 根据 X 列判断右上角按钮（Token/Cost | 设置 | 退出）。
func KindleTopControlAction(x, y int, regions TopHitRegions) string {
	if y < 0 || y >= KindleControlBarYMax {
		return ""
	}
	const padX = 18
	if x >= regions.Exit.X-padX && x < regions.Exit.X+regions.Exit.W+padX {
		return "exit"
	}
	if x >= regions.Settings.X-padX && x < regions.Settings.X+regions.Settings.W+padX {
		return "settings"
	}
	if x >= regions.MetricToggle.X-padX && x < regions.MetricToggle.X+regions.MetricToggle.W+padX {
		return "metric_toggle"
	}
	return ""
}
