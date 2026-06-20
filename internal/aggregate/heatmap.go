package aggregate

import (
	"context"
	"time"
)

func (s *Service) Heatmap(ctx context.Context, providerID string, weeks int) (HeatmapStats, error) {
	var hm HeatmapStats
	if weeks < 1 {
		weeks = 16
	}
	to := time.Now().In(s.loc)
	// 当前周周一，向前 weeks 列（滚动窗口，非自然年）
	thisMonday := to
	for thisMonday.Weekday() != time.Monday {
		thisMonday = thisMonday.AddDate(0, 0, -1)
	}
	from := thisMonday.AddDate(0, 0, -(weeks-1)*7)

	rows, err := s.store.ListDailyRollup(ctx, providerID, from.Format("2006-01-02"), to.Format("2006-01-02"))
	if err != nil {
		return hm, err
	}
	byDate := map[string]int64{}
	var max int64
	for _, r := range rows {
		byDate[r.Date] = r.TotalTokens
		if r.TotalTokens > max {
			max = r.TotalTokens
		}
	}

	hm.Weeks = weeks
	totalDays := weeks * 7
	for i := 0; i < totalDays; i++ {
		d := from.AddDate(0, 0, i)
		key := d.Format("2006-01-02")
		tokens := int64(0)
		if !d.After(to) {
			tokens = byDate[key]
			if tokens > 0 {
				hm.ActiveDays++
				hm.Cumulative += tokens
				if tokens > hm.Peak {
					hm.Peak = tokens
				}
			}
		}
		intensity := 0
		if max > 0 && tokens > 0 {
			ratio := float64(tokens) / float64(max)
			switch {
			case ratio > 0.75:
				intensity = 4
			case ratio > 0.5:
				intensity = 3
			case ratio > 0.25:
				intensity = 2
			case ratio > 0:
				intensity = 1
			}
		}
		hm.Cells = append(hm.Cells, HeatmapCell{Date: key, Intensity: intensity, Tokens: tokens})
	}
	return hm, nil
}
