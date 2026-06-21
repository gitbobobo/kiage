package render

import (
	"testing"
	"time"
)

func TestFormatQuotaReset(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Date(2026, 6, 20, 20, 0, 0, 0, loc)

	cases := []struct {
		name string
		at   time.Time
		want string
	}{
		{
			name: "relative minutes",
			at:   now.Add(45 * time.Minute),
			want: "约 45m 后",
		},
		{
			name: "relative under one minute",
			at:   now.Add(30 * time.Second),
			want: "即将重置",
		},
		{
			name: "absolute same day",
			at:   now.Add(3 * time.Hour),
			want: "23:00",
		},
		{
			name: "absolute next day",
			at:   time.Date(2026, 6, 21, 10, 31, 0, 0, loc),
			want: "6月21日 10:31",
		},
		{
			name: "past",
			at:   now.Add(-time.Minute),
			want: "即将重置",
		},
		{
			name: "zero",
			at:   time.Time{},
			want: "—",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := formatQuotaReset(c.at, now); got != c.want {
				t.Fatalf("got %q want %q", got, c.want)
			}
		})
	}
}

func TestFormatPlanResetRelative(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Date(2026, 6, 20, 20, 0, 0, 0, loc)

	cases := []struct {
		name string
		at   time.Time
		want string
	}{
		{name: "soon", at: now.Add(30 * time.Second), want: "即将重置"},
		{name: "minutes", at: now.Add(22 * time.Minute), want: "22分钟后"},
		{name: "hours", at: now.Add(17 * time.Hour), want: "17小时后"},
		{name: "days", at: now.Add(72 * time.Hour), want: "3天后"},
		{name: "fallback", at: now.Add(20 * 24 * time.Hour), want: "7月10日 20:00"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := formatPlanResetRelative(c.at, now); got != c.want {
				t.Fatalf("got %q want %q", got, c.want)
			}
		})
	}
}
