package render

const heatmapMinCell = 6

// HeatmapWeeksForWidth 按可用行宽计算周列数，使方块尽量大且铺满宽度
func HeatmapWeeksForWidth(width int) int {
	const minWeeks = 12
	for weeks := 53; weeks >= minWeeks; weeks-- {
		if HeatmapCellSizeForWidth(width, weeks) >= heatmapMinCell {
			return weeks
		}
	}
	return minWeeks
}

const heatmapGap = 2

// HeatmapCellSizeForWidth 仅按行宽计算方块边长（铺满宽度）
func HeatmapCellSizeForWidth(width, weeks int) int {
	if weeks < 1 {
		weeks = 1
	}
	cell := (width - heatDayLblW - (weeks-1)*heatmapGap) / weeks
	if cell < heatmapMinCell {
		return heatmapMinCell
	}
	return cell
}

// HeatmapBlockHeight 热力图区域总高度（由方块尺寸决定）
func HeatmapBlockHeight(cellSize int) int {
	return heatMonthH + 7*cellSize + 6*heatmapGap + heatBottomGap + heatBottomTextH
}

// heatmapCellSize 兼容绘制路径：仅按宽度计算
func heatmapCellSize(width, _ int, weeks int) (cellSize, gap int) {
	return HeatmapCellSizeForWidth(width, weeks), heatmapGap
}

// heatmapBlockLayout 计算热力图块（含星期标签）的水平位置与底行 Y
func heatmapBlockLayout(x, y, w, _ int, weeks int) (blockX, gridX, blockW, bottomY int) {
	cellSize, gap := heatmapCellSize(w, 0, weeks)
	step := cellSize + gap
	gridW := weeks*step - gap
	blockW = heatDayLblW + gridW
	blockX = x
	gridX = blockX + heatDayLblW
	bottomY = y + HeatmapBlockHeight(cellSize) - heatBottomTextH
	return blockX, gridX, blockW, bottomY
}
