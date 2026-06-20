package syncer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/godbobo/kiage/internal/provider"
	"github.com/godbobo/kiage/internal/store"
)

type Progress struct {
	Mode    string
	Message string
	Percent float64
}

type Service struct {
	mu       sync.Mutex
	prov     provider.Provider
	store    *store.Store
	onProg   func(Progress)
	pageSize int
	delay    time.Duration
}

func New(prov provider.Provider, st *store.Store) *Service {
	return &Service{
		prov:     prov,
		store:    st,
		pageSize: 100,
		delay:    200 * time.Millisecond,
	}
}

func (s *Service) OnProgress(fn func(Progress)) { s.onProg = fn }

func (s *Service) Run(ctx context.Context, mode string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.runLocked(ctx, mode)
}

func (s *Service) runLocked(ctx context.Context, mode string) error {
	loc := s.prov.Timezone()
	now := time.Now().In(loc)
	year := now.Year()
	yearKey := fmt.Sprintf("%d", year)

	syncedYear, _, _ := s.store.GetState(ctx, s.prov.ID(), "last_full_sync_year")
	needFull := mode == "full" || syncedYear != yearKey

	if needFull {
		if err := s.fullSync(ctx, year, now); err != nil {
			return err
		}
		_ = s.store.SetState(ctx, s.prov.ID(), "last_full_sync_year", yearKey)
	}

	if err := s.incrementalToday(ctx, now); err != nil {
		return err
	}

	sum, err := s.prov.FetchSummary(ctx)
	if err != nil {
		return err
	}
	if err := s.store.SaveSummary(ctx, s.prov.ID(), sum); err != nil {
		return err
	}

	if err := s.validateAndRepair(ctx, sum, loc); err != nil {
		return err
	}

	_ = s.store.SetState(ctx, s.prov.ID(), "last_successful_sync_at", time.Now().UTC().Format(time.RFC3339))
	return nil
}

func (s *Service) fullSync(ctx context.Context, year int, now time.Time) error {
	loc := s.prov.Timezone()
	start := time.Date(year, 1, 1, 0, 0, 0, 0, loc)
	endYesterday := startOfDay(now, loc).Add(-time.Nanosecond)

	s.report(Progress{Mode: "full", Message: "补全当年历史数据", Percent: 0})

	if endYesterday.Before(start) {
		return nil
	}

	totalDays := int(endYesterday.Sub(start).Hours()/24) + 1
	day := 0
	for d := start; !d.After(endYesterday); d = d.AddDate(0, 0, 1) {
		day++
		rng := provider.DateRange{
			Start: d,
			End:   endOfDay(d, loc),
		}
		if err := s.fetchRange(ctx, rng); err != nil {
			return err
		}
		if err := s.store.RebuildDailyRollup(ctx, s.prov.ID(), d.Format("2006-01-02")); err != nil {
			return err
		}
		pct := float64(day) / float64(totalDays) * 100
		s.report(Progress{Mode: "full", Message: fmt.Sprintf("补全 %s", d.Format("01-02")), Percent: pct})
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	return nil
}

func (s *Service) incrementalToday(ctx context.Context, now time.Time) error {
	loc := s.prov.Timezone()
	today := startOfDay(now, loc)
	rng := provider.DateRange{Start: today, End: endOfDay(today, loc)}
	s.report(Progress{Mode: "incremental", Message: "刷新今日数据", Percent: 50})
	if err := s.fetchRange(ctx, rng); err != nil {
		return err
	}
	return s.store.RebuildDailyRollup(ctx, s.prov.ID(), today.Format("2006-01-02"))
}

func (s *Service) fetchRange(ctx context.Context, rng provider.DateRange) error {
	page := 1
	for {
		ep, err := s.prov.FetchUsageEvents(ctx, rng, page, s.pageSize)
		if err != nil {
			return err
		}
		for _, ev := range ep.Events {
			cs := checksum(ev)
			if err := s.store.UpsertEvent(ctx, s.prov.ID(), ev, cs); err != nil {
				return err
			}
		}
		if !ep.HasMore {
			break
		}
		page++
		time.Sleep(s.delay)
	}
	return nil
}

func (s *Service) validateAndRepair(ctx context.Context, sum provider.Summary, loc *time.Location) error {
	from := sum.BillingCycleStart.In(loc).Format("2006-01-02")
	to := time.Now().In(loc).Format("2006-01-02")
	_, cost, err := s.store.SumRollupRange(ctx, s.prov.ID(), from, to)
	if err != nil {
		return err
	}
	// usage-summary doesn't expose exact monthly cost; skip token-only plans
	if sum.OnDemandUsedCents > 0 && cost > 0 {
		diff := abs(cost - sum.OnDemandUsedCents)
		if sum.OnDemandUsedCents > 0 && diff/sum.OnDemandUsedCents > 0.05 {
			return s.repairLastDays(ctx, loc, 7)
		}
	}
	return nil
}

func (s *Service) repairLastDays(ctx context.Context, loc *time.Location, days int) error {
	now := time.Now().In(loc)
	for i := 0; i < days; i++ {
		d := startOfDay(now.AddDate(0, 0, -i), loc)
		rng := provider.DateRange{Start: d, End: endOfDay(d, loc)}
		if err := s.fetchRange(ctx, rng); err != nil {
			return err
		}
		_ = s.store.RebuildDailyRollup(ctx, s.prov.ID(), d.Format("2006-01-02"))
	}
	return nil
}

func checksum(ev provider.UsageEvent) string {
	raw := fmt.Sprintf("%s|%s|%d|%d|%f", ev.Timestamp.UTC().Format(time.RFC3339Nano), ev.Model, ev.TotalTokens, ev.OutputTokens, ev.CostCents)
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:8])
}

func startOfDay(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

func endOfDay(t time.Time, loc *time.Location) time.Time {
	return startOfDay(t, loc).Add(24*time.Hour - time.Nanosecond)
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func (s *Service) report(p Progress) {
	if s.onProg != nil {
		s.onProg(p)
	}
}
