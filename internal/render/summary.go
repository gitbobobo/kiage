package render

import (
	"image"
	"image/color"
	"image/draw"
	"time"

	"github.com/godbobo/kiage/internal/aggregate"
	"github.com/godbobo/kiage/internal/provider"
)

const (
	summaryProviderGap = 12
	summaryBarGap      = 2
)

// DrawSummaryFrame 绘制用量概览首页。
func DrawSummaryFrame(overview aggregate.Overview, view ViewState, size Size) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size.Width, size.Height))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

	w := size.Width - PadX*2
	x := PadX
	cy := 16

	drawTopRightControls(img, x+w, cy+2, view)
	cy = drawText(img, x, cy, SummaryPageTitle, TitleFontSize(), true)

	contentTop := cy + 12
	contentBottom := size.Height - 16

	type blockPlan struct {
		ps   aggregate.ProviderSummary
		bars []provider.QuotaBar
	}
	plans := make([]blockPlan, 0, len(overview.Providers))
	for _, ps := range overview.Providers {
		plans = append(plans, blockPlan{
			ps:   ps,
			bars: BarsForProvider(ps.ProviderID, ps.Dashboard, ps.Configured),
		})
	}

	cy = contentTop
	now := time.Now()
	remaining := 0
	for i, plan := range plans {
		if cy >= contentBottom {
			remaining = len(plans) - i
			break
		}
		used, hidden := drawProviderBlock(img, x, cy, w, contentBottom, plan.ps, plan.bars, now)
		if hidden > 0 {
			remaining += hidden
		}
		cy += used
		if i < len(plans)-1 {
			cy += summaryProviderGap
		}
	}
	if remaining > 0 {
		msg := "…"
		if remaining > 1 {
			msg = "还有 " + itoa(remaining) + " 项未展示"
		}
		drawText(img, x, contentBottom-18, msg, StatusFontSize(), false)
	}
	return img
}

func barCompactHeight() int {
	return 12 + 4 + 8 // fontSize + gap + barH
}

func drawProviderBlock(img *image.RGBA, x, y, w, maxY int, ps aggregate.ProviderSummary, bars []provider.QuotaBar, now time.Time) (used int, hiddenBars int) {
	cy := y
	if cy >= maxY {
		return 0, 0
	}
	name := ps.DisplayName
	if name == "" {
		name = "—"
	}
	cy = drawText(img, x, cy, name, TitleFontSize(), true)

	updated := "—"
	if ps.Configured {
		if ps.LastUpdatedAt.IsZero() {
			updated = "未同步"
		} else {
			updated = formatUpdatedAt(ps.LastUpdatedAt)
		}
	}
	status := ps.SyncStatus
	if status == "" {
		status = "就绪"
	}
	cy = drawText(img, x, cy+6, "更新于 "+updated+" · "+status, StatusFontSize(), false)

	plan := ps.PlanName
	if !ps.Configured {
		plan = "—"
	}
	cy = drawText(img, x, cy+8, FormatPlanLine(ps.ProviderID, plan, ps.MembershipType, ps.ResetAt, bars, now), PlanFontSize(), true)
	cy += 8

	if len(bars) == 0 {
		if cy+16 <= maxY {
			cy = drawText(img, x, cy, "—", StatusFontSize(), false)
		}
		return cy - y, 0
	}
	for i, bar := range bars {
		bh := barCompactHeight()
		if cy+bh > maxY {
			hiddenBars = len(bars) - i
			break
		}
		cy = drawBarCompact(img, x, cy, w, bar.Label, bar.Percent)
		if i < len(bars)-1 {
			cy += summaryBarGap
		}
	}
	return cy - y, hiddenBars
}

func drawBarCompact(img *image.RGBA, x, y, w int, label string, pct float64) int {
	const fontSize = 12
	const barH = 8
	drawText(img, x, y, label+" "+formatPct(pct), fontSize, false)
	barY := y + fontSize + 4
	fillW := int(float64(w) * clamp(pct/100, 0, 1))
	drawRect(img, x, barY, w, barH, color.Gray{Y: 220})
	if fillW > 0 {
		drawRect(img, x, barY, fillW, barH, color.Gray{Y: 40})
	}
	return barY + barH
}
