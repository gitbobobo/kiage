package render

import (
	"image"
	"image/color"
	"time"

	"github.com/godbobo/kiage/internal/aggregate"
)

func drawTopSection(img *image.RGBA, dash aggregate.Dashboard, view ViewState, x, y, w, h int) {
	_ = h
	cy := y
	drawTopRightControls(img, x+w, y+2, view.ChartMetric, view.SettingsActive, view.SettingsURL)
	title := view.ProviderName
	if title == "" {
		title = "—"
	}
	cy = drawText(img, x, cy, title, TitleFontSize(), true)
	status := view.SyncStatus
	if status == "" {
		status = "就绪"
	}
	updated := "尚未更新"
	if !dash.LastUpdatedAt.IsZero() {
		updated = formatUpdatedAt(dash.LastUpdatedAt)
	}
	cy = drawText(img, x, cy+6, "更新于 "+updated+" · "+status, StatusFontSize(), false)

	plan := dash.PlanName
	if plan == "" {
		plan = "—"
	}
	reset := "—"
	if !dash.ResetAt.IsZero() {
		reset = dash.ResetAt.Format("1月2日") + " (" + itoa(dash.ResetDaysLeft) + "天)"
	}
	cy = drawText(img, x, cy+8, "套餐 "+plan+" · 重置 "+reset, PlanFontSize(), true)

	cy += 8
	cy = drawBar(img, x, cy, w, "Total", dash.TotalPercent)
	cy = drawBar(img, x, cy+4, w, "Composer", dash.ComposerPercent)
	cy = drawBar(img, x, cy+4, w, "API", dash.APIPercent)

	cy += 12
	colW := w / 3
	boxH := PeriodBoxHeight()
	periods := []struct {
		title string
		value string
	}{
		{"今日", formatPeriodValue(view.ChartMetric, dash.DayTokens, dash.DayCost)},
		{"本月", formatPeriodValue(view.ChartMetric, dash.MonthTokens, dash.MonthCost)},
		{"今年", formatPeriodValue(view.ChartMetric, dash.YearTokens, dash.YearCost)},
	}
	for i, p := range periods {
		drawPeriodBox(img, x+i*colW, cy, colW-6, boxH, p.title, p.value)
	}
}

func formatUpdatedAt(t time.Time) string {
	t = t.In(time.Local)
	now := time.Now()
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return t.Format("15:04")
	}
	return t.Format("1月2日 15:04")
}

func formatPeriodValue(metric string, tokens int64, cost float64) string {
	if metric == "cost" {
		return aggregate.FormatCost(cost)
	}
	return aggregate.FormatTokens(tokens)
}

func drawMetricToggle(img *image.RGBA, rightX, y int, metric string) {
	segH := MetricToggleHeight()
	fontSize := MetricToggleFontSize()
	padX := MetricTogglePadX()
	gap := 2
	if KindleUI() {
		gap = 4
	}
	opts := []struct {
		key   string
		label string
	}{
		{"token", "Token"},
		{"cost", "Cost"},
	}
	segW := make([]int, len(opts))
	totalW := 0
	for i, o := range opts {
		segW[i] = textWidth(o.label, fontSize) + padX*2
		totalW += segW[i]
	}
	totalW += gap
	x := rightX - totalW
	for i, o := range opts {
		sx := x
		for j := 0; j < i; j++ {
			sx += segW[j] + gap
		}
		sw := segW[i]
		active := metric == o.key
		bg := color.Gray{Y: 235}
		if active {
			bg = color.Gray{Y: 60}
		}
		drawRect(img, sx, y, sw, segH, bg)
		if active {
			drawRectOutline(img, sx, y, sw, segH, color.Gray{Y: 20})
		}
		labelW := textWidth(o.label, fontSize)
		labelX := sx + (sw-labelW)/2
		textColor := color.Black
		if active {
			textColor = color.White
		}
		drawTextColor(img, labelX, y+8, o.label, fontSize, false, textColor)
	}
}

func drawPeriodBox(img *image.RGBA, x, y, w, h int, title, value string) {
	drawRect(img, x, y, w, h, color.Gray{Y: 225})
	padX := 12
	if KindleUI() {
		padX = 14
	}
	titleSz := PeriodTitleFontSize()
	valueSz := PeriodValueFontSize()
	drawText(img, x+padX, y+10, title, titleSz, false)
	drawText(img, x+padX, y+10+titleSz+8, value, valueSz, true)
}

func drawRectOutline(img *image.RGBA, x, y, w, h int, c color.Color) {
	for dx := 0; dx < w; dx++ {
		img.Set(x+dx, y, c)
		img.Set(x+dx, y+h-1, c)
	}
	for dy := 0; dy < h; dy++ {
		img.Set(x, y+dy, c)
		img.Set(x+w-1, y+dy, c)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
