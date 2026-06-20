package render

import (
	"image"
	"image/color"
	"time"

	"github.com/godbobo/kiage/internal/aggregate"
)

func drawHeatmap(img *image.RGBA, x, y, w, h int, heat aggregate.HeatmapStats) {
	if len(heat.Cells) == 0 {
		drawChartFrame(img, x, y, w, h, "暂无数据")
		return
	}

	weeks := heat.Weeks
	if weeks <= 0 {
		weeks = len(heat.Cells) / 7
	}

	cellSize, gap := heatmapCellSize(w, h, weeks)
	step := cellSize + gap

	blockX, gridX, blockW, bottomY := heatmapBlockLayout(x, y, w, h, weeks)

	contentY := y + heatMonthH
	var lastMonth time.Month
	for col := 0; col < weeks; col++ {
		idx := col * 7
		if idx >= len(heat.Cells) {
			break
		}
		t, err := time.Parse("2006-01-02", heat.Cells[idx].Date)
		if err != nil {
			continue
		}
		if t.Month() != lastMonth {
			lx := gridX + col*step
			label := t.Format("1月")
			if lx >= x && lx+textWidth(label, 11) <= x+w {
				drawText(img, lx, y, label, 11, false)
			}
			lastMonth = t.Month()
		}
	}

	dayLabels := []struct {
		row int
		lbl string
	}{
		{0, "一"},
		{2, "三"},
		{4, "五"},
	}
	for _, dl := range dayLabels {
		drawText(img, blockX, contentY+dl.row*step+2, dl.lbl, 10, false)
	}

	levels := []color.Gray{{Y: 245}, {Y: 210}, {Y: 170}, {Y: 110}, {Y: 45}}
	for i, c := range heat.Cells {
		col := i / 7
		row := i % 7
		if col >= weeks {
			break
		}
		cx := gridX + col*step
		cy := contentY + row*step
		if cy+cellSize > bottomY-heatBottomGap {
			continue
		}
		shade := levels[clampInt(c.Intensity, 0, 4)]
		drawRoundCell(img, cx, cy, cellSize, shade)
	}

	// 底部左侧：图例（与热力图块左对齐）
	legCell := 9
	legendX := blockX
	drawText(img, legendX, bottomY, "少", 10, false)
	for i := 0; i < 5; i++ {
		drawRoundCell(img, legendX+14+i*(legCell+3), bottomY, legCell, levels[i])
	}
	drawText(img, legendX+14+5*(legCell+3)+4, bottomY, "多", 10, false)

	// 底部右侧：统计（与热力图块右对齐）
	statsLine := "累计 " + aggregate.FormatTokens(heat.Cumulative) +
		"  峰值 " + aggregate.FormatTokens(heat.Peak) +
		"  活跃 " + itoa(heat.ActiveDays) + "天"
	drawTextRight(img, blockX+blockW, bottomY, statsLine, 11, false)
}

func drawRoundCell(img *image.RGBA, x, y, size int, c color.Color) {
	r := size / 4
	if r < 1 {
		r = 1
	}
	for dy := 0; dy < size; dy++ {
		for dx := 0; dx < size; dx++ {
			if dx < r && dy < r && (r-dx)*(r-dx)+(r-dy)*(r-dy) > r*r {
				continue
			}
			if dx >= size-r && dy < r && (dx-(size-r-1))*(dx-(size-r-1))+(r-dy)*(r-dy) > r*r {
				continue
			}
			if dx < r && dy >= size-r && (r-dx)*(r-dx)+(dy-(size-r-1))*(dy-(size-r-1)) > r*r {
				continue
			}
			if dx >= size-r && dy >= size-r && (dx-(size-r-1))*(dx-(size-r-1))+(dy-(size-r-1))*(dy-(size-r-1)) > r*r {
				continue
			}
			img.Set(x+dx, y+dy, c)
		}
	}
}
