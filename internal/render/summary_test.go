package render

import (
	"image"
	"testing"
	"time"

	"github.com/godbobo/kiage/internal/aggregate"
	"github.com/godbobo/kiage/internal/provider"
)

func TestFormatPlanLineCursor(t *testing.T) {
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.Local)
	reset := time.Date(2026, 6, 30, 0, 0, 0, 0, time.Local)
	got := FormatPlanLine(provider.CursorID, "Pro", "", reset, nil, now)
	if got != "套餐 Pro · 重置 6月30日 00:00" {
		t.Fatalf("unexpected: %q", got)
	}
}

func TestFormatPlanLineMiniMax(t *testing.T) {
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.Local)
	reset := time.Date(2026, 6, 21, 18, 30, 0, 0, time.Local)
	got := FormatPlanLine(provider.MiniMaxID, "Token Plan", "interval", reset, nil, now)
	if got != "套餐 Token Plan · 时段重置 18:30" {
		t.Fatalf("unexpected: %q", got)
	}
}

func TestFormatPlanLineMiniMaxWeekly(t *testing.T) {
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.Local)
	reset := time.Date(2026, 6, 28, 0, 0, 0, 0, time.Local)
	got := FormatPlanLine(provider.MiniMaxID, "Token Plan", "weekly", reset, nil, now)
	if got != "套餐 Token Plan · 周重置 6月28日 00:00" {
		t.Fatalf("unexpected: %q", got)
	}
}

func TestFormatPlanLineKimi(t *testing.T) {
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.Local)
	bars := []provider.QuotaBar{
		{Label: provider.LabelIntervalQuota, ResetAt: now.Add(22 * time.Minute)},
		{Label: provider.LabelWeeklyQuota, ResetAt: now.Add(17 * time.Hour)},
	}
	got := FormatPlanLine(provider.KimiID, "Moderato", "LEVEL_INTERMEDIATE", time.Time{}, bars, now)
	if got != "套餐 Moderato · 时段重置 12:22 · 周重置 6月22日 05:00" {
		t.Fatalf("unexpected: %q", got)
	}
}

func TestBarsForProviderUnconfigured(t *testing.T) {
	if BarsForProvider(provider.CursorID, aggregate.Dashboard{TotalPercent: 50}, false) != nil {
		t.Fatal("unconfigured should return nil bars")
	}
	if BarsForProvider(provider.MiniMaxID, aggregate.Dashboard{}, true) != nil {
		t.Fatal("configured with empty bars should return nil")
	}
}

func TestDrawProviderBlockCompactHeight(t *testing.T) {
	t.Setenv("KIAGE_PORTRAIT", "1")
	img := image.NewRGBA(image.Rect(0, 0, 400, 2000))
	now := time.Now()
	ps := aggregate.ProviderSummary{
		ProviderID:  provider.CursorID,
		DisplayName: "Cursor",
		Configured:  true,
		Dashboard: aggregate.Dashboard{
			PlanName:      "Ultra",
			SyncStatus:    "就绪",
			LastUpdatedAt: now,
			Bars: []provider.QuotaBar{
				{Label: "Total", Percent: 73},
				{Label: "Composer", Percent: 59},
				{Label: "API", Percent: 100},
			},
		},
	}
	used, hidden := drawProviderBlock(img, 28, 100, 300, 2000, ps, ps.Bars, now)
	if hidden != 0 {
		t.Fatalf("unexpected hidden bars: %d", hidden)
	}
	if used > 260 {
		t.Fatalf("provider block too tall: %d", used)
	}
}
