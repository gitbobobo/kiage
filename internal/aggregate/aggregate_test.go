package aggregate

import (
	"context"
	"testing"
	"time"

	"github.com/godbobo/kiage/internal/provider"
	"github.com/godbobo/kiage/internal/store"
)

func TestBuildMonthRangeUsesCalendarWhenBillingStartZero(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	st, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ctx := context.Background()
	now := time.Now().In(loc)
	today := now.Format("2006-01-02")
	if err := st.UpsertEvent(ctx, provider.GLMID, provider.UsageEvent{
		EventID: "old", Timestamp: time.Date(2020, 1, 1, 12, 0, 0, 0, loc),
		LocalDate: "2020-01-01", TotalTokens: 999,
	}, "x"); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertEvent(ctx, provider.GLMID, provider.UsageEvent{
		EventID: "today", Timestamp: now, LocalDate: today, TotalTokens: 42,
	}, "y"); err != nil {
		t.Fatal(err)
	}
	if err := st.RebuildDailyRollup(ctx, provider.GLMID, "2020-01-01"); err != nil {
		t.Fatal(err)
	}
	if err := st.RebuildDailyRollup(ctx, provider.GLMID, today); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveSummary(ctx, provider.GLMID, provider.Summary{
		PlanName: "Max",
		Bars:     []provider.QuotaBar{{Label: "5小时配额", Percent: 1}},
	}); err != nil {
		t.Fatal(err)
	}

	svc := New(st, loc)
	dash, err := svc.Build(ctx, provider.GLMID)
	if err != nil {
		t.Fatal(err)
	}
	if dash.MonthTokens != 42 {
		t.Fatalf("want month tokens 42 got %d", dash.MonthTokens)
	}
}
