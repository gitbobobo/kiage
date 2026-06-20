package render

import "testing"

func TestHeatmapFillsWidth(t *testing.T) {
	width := 1016 // Oasis 竖屏内容区
	weeks := HeatmapWeeksForWidth(width)
	cell := HeatmapCellSizeForWidth(width, weeks)
	gridW := weeks*cell + (weeks-1)*heatmapGap
	blockW := heatDayLblW + gridW
	slack := width - blockW
	if slack < 0 || slack > weeks {
		t.Fatalf("weeks=%d cell=%d blockW=%d slack=%d want near full width", weeks, cell, blockW, slack)
	}
	if cell < 10 {
		t.Fatalf("cell=%d too small for width=%d", cell, width)
	}
}

func TestHeatmapHeightFromCell(t *testing.T) {
	cell := 14
	h := HeatmapBlockHeight(cell)
	want := heatMonthH + 7*cell + 6*heatmapGap + heatBottomGap + heatBottomTextH
	if h != want {
		t.Fatalf("height=%d want %d", h, want)
	}
}
