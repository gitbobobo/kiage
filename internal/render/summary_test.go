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
	got := FormatPlanLine(provider.CursorID, "Pro", reset, 9, now)
	if got != "套餐 Pro · 重置 6月30日 (9天)" {
		t.Fatalf("unexpected: %q", got)
	}
}

func TestBarsForProviderUnconfigured(t *testing.T) {
	if BarsForProvider(provider.CursorID, aggregate.Dashboard{TotalPercent: 50}, false) != nil {
		t.Fatal("unconfigured should return nil bars")
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
	// Kindle UI 标题区约 150px，3 条紧凑配额条约 80px
	if used > 260 {
		t.Fatalf("provider block too tall: %d", used)
	}
}
