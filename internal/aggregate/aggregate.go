package aggregate

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/godbobo/kiage/internal/provider"
	"github.com/godbobo/kiage/internal/store"
)

type Dashboard struct {
	PlanName        string
	ResetAt         time.Time
	ResetDaysLeft   int
	Bars            []provider.QuotaBar
	TotalPercent    float64
	ComposerPercent float64
	APIPercent      float64
	MonthTokens     int64
	MonthCost       float64
	DayTokens       int64
	DayCost         float64
	YearTokens      int64
	YearCost        float64
	SummaryFetched  time.Time
	LastUpdatedAt   time.Time // 上次成功拉取数据的时间
	SyncStatus      string
	SyncMessage     string
}

type LinePoint struct {
	Date   string
	Tokens int64
	Cost   float64
}

type HeatmapCell struct {
	Date      string
	Intensity int // 0-4
	Tokens    int64
}

type HeatmapStats struct {
	Cumulative int64
	Peak       int64
	ActiveDays int
	Cells      []HeatmapCell
	Weeks      int
}

type Service struct {
	store *store.Store
	loc   *time.Location
}

func New(st *store.Store, loc *time.Location) *Service {
	return &Service{store: st, loc: loc}
}

func (s *Service) Build(ctx context.Context, providerID string) (Dashboard, error) {
	var dash Dashboard
	now := time.Now().In(s.loc)
	sum, ok, err := s.store.LoadSummary(ctx, providerID)
	if err != nil {
		return dash, err
	}
	if ok {
		dash.PlanName = sum.PlanName
		dash.ResetAt = sum.ResetAt
		dash.Bars = sum.Bars
		dash.TotalPercent = sum.TotalPercent
		dash.ComposerPercent = sum.ComposerPercent
		dash.APIPercent = sum.APIPercent
		dash.SummaryFetched = sum.FetchedAt
		dash.ResetDaysLeft = int(math.Ceil(sum.ResetAt.Sub(now).Hours() / 24))
		if dash.ResetDaysLeft < 0 {
			dash.ResetDaysLeft = 0
		}

		mFrom := sum.BillingStart.In(s.loc).Format("2006-01-02")
		if sum.BillingStart.IsZero() {
			mFrom = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, s.loc).Format("2006-01-02")
		}
		mTo := now.Format("2006-01-02")
		dash.MonthTokens, dash.MonthCost, _ = s.store.SumRollupRange(ctx, providerID, mFrom, mTo)
	}

	today := now.Format("2006-01-02")
	dash.DayTokens, dash.DayCost, _ = s.store.SumRollupRange(ctx, providerID, today, today)

	yearStart := time.Date(nowYear(s.loc), 1, 1, 0, 0, 0, 0, s.loc).Format("2006-01-02")
	yearEnd := now.Format("2006-01-02")
	dash.YearTokens, dash.YearCost, _ = s.store.SumRollupRange(ctx, providerID, yearStart, yearEnd)

	if v, ok, _ := s.store.GetState(ctx, providerID, "last_successful_sync_at"); ok {
		dash.LastUpdatedAt, _ = time.Parse(time.RFC3339, v)
	}
	return dash, nil
}

func (s *Service) LineSeries(ctx context.Context, providerID string, days int) ([]LinePoint, error) {
	to := time.Now().In(s.loc)
	from := to.AddDate(0, 0, -(days - 1))
	rows, err := s.store.ListDailyRollup(ctx, providerID, from.Format("2006-01-02"), to.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	byDate := map[string]store.DailyRollup{}
	for _, r := range rows {
		byDate[r.Date] = r
	}
	out := make([]LinePoint, 0, days)
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		r := byDate[key]
		out = append(out, LinePoint{Date: key, Tokens: r.TotalTokens, Cost: r.TotalCost})
	}
	return out, nil
}

func FormatTokens(n int64) string {
	switch {
	case n >= 1_000_000_000:
		return fmt.Sprintf("%.2fB", float64(n)/1e9)
	case n >= 1_000_000:
		return fmt.Sprintf("%.2fM", float64(n)/1e6)
	case n >= 10_000:
		return fmt.Sprintf("%.1f万", float64(n)/1e4)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func FormatCost(cents float64) string {
	return fmt.Sprintf("$%.2f", cents/100)
}

func nowYear(loc *time.Location) int {
	return time.Now().In(loc).Year()
}
