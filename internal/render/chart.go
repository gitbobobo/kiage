package render

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"time"

	"github.com/godbobo/kiage/internal/aggregate"
)

type chartLayout struct {
	plotX, plotY, plotW, plotH int
}

func chartLayoutFor(x, y, w, h int) chartLayout {
	padding := 50
	plotX := x + padding
	plotY := y + 22
	plotW := w - padding - 12
	plotH := h - 56
	return chartLayout{plotX, plotY, plotW, plotH}
}

// drawLineChart renders a daily trend chart. Returns nil even when there is no
// data so the chart frame and axes remain visible.
func drawLineChart(img *image.RGBA, x, y, w, h int, points []aggregate.LinePoint, metric string) error {
	layout := chartLayoutFor(x, y, w, h)
	if layout.plotW < 10 || layout.plotH < 10 {
		return nil
	}

	vals := make([]float64, len(points))
	for i, p := range points {
		if metric == "cost" {
			vals[i] = p.Cost / 100
		} else {
			vals[i] = float64(p.Tokens)
		}
	}

	if len(vals) == 0 {
		drawChartFrame(img, x, y, w, h, "暂无数据")
		return nil
	}

	maxV := 0.0
	for _, v := range vals {
		if v > maxV {
			maxV = v
		}
	}
	if maxV <= 0 {
		drawChartFrame(img, x, y, w, h, "暂无用量数据")
		return nil
	}

	scale := 1.0
	unit := ""
	if metric != "cost" {
		if maxV >= 1_000_000 {
			scale = 1e6
			unit = "M"
		} else if maxV >= 10_000 {
			scale = 1e4
			unit = "万"
		}
	}
	scaled := make([]float64, len(vals))
	for i, v := range vals {
		scaled[i] = v / scale
	}
	maxScaled := maxV / scale

	drawChartGrid(img, layout)
	drawChartYTicks(img, layout, maxScaled, metric, unit)
	drawChartXTicks(img, layout, points)

	title := "30d"
	if metric == "cost" {
		title = "30d Cost ($)"
	} else if unit != "" {
		title = fmt.Sprintf("30d Token (%s)", unit)
	} else {
		title = "30d Token"
	}
	drawText(img, x+8, y+4, title, 14, true)

	n := len(scaled)
	if n == 1 {
		px := layout.plotX + layout.plotW/2
		py := layout.plotY + layout.plotH/2
		drawRect(img, px-2, py-2, 5, 5, color.Black)
	} else {
		xs := make([]int, n)
		ys := make([]int, n)
		for i, v := range scaled {
			xs[i] = layout.plotX + (layout.plotW * i / (n - 1))
			ys[i] = layout.plotY + layout.plotH - int(float64(layout.plotH)*v/maxScaled)
		}
		drawChartAreaFill(img, layout, xs, ys, color.Gray{Y: 245})
		for i := 1; i < n; i++ {
			drawThickLine(img, xs[i-1], ys[i-1], xs[i], ys[i])
		}
	}
	return nil
}

func drawChartAreaFill(img *image.RGBA, l chartLayout, xs, ys []int, fill color.Color) {
	if len(xs) < 2 {
		return
	}
	baseline := l.plotY + l.plotH
	for x := xs[0]; x <= xs[len(xs)-1]; x++ {
		if x < l.plotX || x > l.plotX+l.plotW {
			continue
		}
		yTop := interpolateLineY(x, xs, ys)
		if yTop < l.plotY {
			yTop = l.plotY
		}
		if yTop > baseline {
			yTop = baseline
		}
		for y := yTop; y <= baseline; y++ {
			img.Set(x, y, fill)
		}
	}
}

func interpolateLineY(x int, xs, ys []int) int {
	if x <= xs[0] {
		return ys[0]
	}
	last := len(xs) - 1
	if x >= xs[last] {
		return ys[last]
	}
	for i := 0; i < last; i++ {
		if x >= xs[i] && x <= xs[i+1] {
			if xs[i+1] == xs[i] {
				return ys[i]
			}
			t := float64(x-xs[i]) / float64(xs[i+1]-xs[i])
			return int(float64(ys[i]) + t*float64(ys[i+1]-ys[i]))
		}
	}
	return ys[last]
}

func drawChartFrame(img *image.RGBA, x, y, w, h int, msg string) {
	layout := chartLayoutFor(x, y, w, h)
	if layout.plotW < 10 || layout.plotH < 10 {
		drawText(img, x+8, y+8, msg, 14, false)
		return
	}
	drawChartGrid(img, layout)
	drawChartYTicks(img, layout, 0, "", "")
	drawText(img, layout.plotX+8, layout.plotY+layout.plotH/2, msg, 14, false)
}

func drawChartGrid(img *image.RGBA, l chartLayout) {
	for i := 0; i <= 4; i++ {
		gy := l.plotY + l.plotH*i/4
		drawHLine(img, l.plotX, gy, l.plotW, color.Gray{Y: 230})
	}
	drawVLine(img, l.plotX, l.plotY, l.plotH, color.Gray{Y: 180})
	drawHLine(img, l.plotX, l.plotY+l.plotH, l.plotW, color.Gray{Y: 180})
}

func drawChartYTicks(img *image.RGBA, l chartLayout, maxScaled float64, metric, unit string) {
	for i := 0; i <= 4; i++ {
		gy := l.plotY + l.plotH - l.plotH*i/4
		if i == 0 || i == 4 {
			// 底部 0 与顶部刻度省略数值，避免与 X 轴日期 / 标题重叠
			for dx := -4; dx < 0; dx++ {
				img.Set(l.plotX+dx, gy, color.Gray{Y: 180})
			}
			continue
		}
		val := maxScaled * float64(i) / 4
		label := formatChartY(val, metric, unit)
		drawTextRight(img, l.plotX-8, gy-4, label, 11, false)
		for dx := -4; dx < 0; dx++ {
			img.Set(l.plotX+dx, gy, color.Gray{Y: 180})
		}
	}
}

func chartXTicksCount(n, plotW int) int {
	ticks := plotW / 42
	if ticks < 6 {
		ticks = 6
	}
	if ticks > 10 {
		ticks = 10
	}
	if n < ticks {
		return n
	}
	return ticks
}

func drawChartXTicks(img *image.RGBA, l chartLayout, points []aggregate.LinePoint) {
	n := len(points)
	if n == 0 {
		return
	}
	ticks := chartXTicksCount(n, l.plotW)
	for i := 0; i < ticks; i++ {
		idx := 0
		if ticks > 1 {
			idx = i * (n - 1) / (ticks - 1)
		}
		px := l.plotX
		if ticks > 1 {
			px = l.plotX + l.plotW*i/(ticks-1)
		}
		label := formatChartX(points[idx].Date)
		lw := textWidth(label, 11)
		lx := px - lw/2
		switch {
		case i == 0:
			lx = px + 2
		case i == ticks-1:
			lx = px - lw - 2
		}
		drawText(img, lx, l.plotY+l.plotH+8, label, 11, false)
		for dy := 0; dy <= 3; dy++ {
			img.Set(px, l.plotY+l.plotH+dy, color.Gray{Y: 180})
		}
	}
}

func formatChartY(v float64, metric, unit string) string {
	if metric == "cost" {
		return fmt.Sprintf("%.1f", v)
	}
	if unit == "M" {
		if v >= 100 {
			return fmt.Sprintf("%.0f", v)
		}
		return fmt.Sprintf("%.1f", v)
	}
	if unit == "万" {
		return fmt.Sprintf("%.0f", v)
	}
	if v >= 1_000_000 {
		return fmt.Sprintf("%.0fM", v/1e6)
	}
	if v >= 10_000 {
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%.0f", v)
}

func formatChartX(date string) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		if len(date) >= 10 {
			return date[5:10]
		}
		return date
	}
	return fmt.Sprintf("%d/%d", int(t.Month()), t.Day())
}

func drawHLine(img *image.RGBA, x, y, w int, c color.Color) {
	for dx := 0; dx < w; dx++ {
		img.Set(x+dx, y, c)
	}
}

func drawVLine(img *image.RGBA, x, y, h int, c color.Color) {
	for dy := 0; dy < h; dy++ {
		img.Set(x, y+dy, c)
	}
}

func drawThickLine(img *image.RGBA, x0, y0, x1, y1 int) {
	dx := math.Abs(float64(x1 - x0))
	dy := math.Abs(float64(y1 - y0))
	steps := int(math.Max(dx, dy))
	if steps == 0 {
		img.Set(x0, y0, color.Black)
		return
	}
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		x := int(float64(x0) + t*float64(x1-x0))
		y := int(float64(y0) + t*float64(y1-y0))
		for oy := -1; oy <= 1; oy++ {
			for ox := -1; ox <= 1; ox++ {
				img.Set(x+ox, y+oy, color.Black)
			}
		}
	}
}
