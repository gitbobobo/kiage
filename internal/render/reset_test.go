package render

import (
	"testing"
	"time"
)

func TestFormatResetAbsolute(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Date(2026, 6, 20, 20, 0, 0, 0, loc)

	cases := []struct {
		name string
		at   time.Time
		want string
	}{
		{
			name: "same day later",
			at:   now.Add(3 * time.Hour),
			want: "23:00",
		},
		{
			name: "same day soon",
			at:   now.Add(30 * time.Second),
			want: "20:00",
		},
		{
			name: "next day",
			at:   time.Date(2026, 6, 21, 10, 31, 0, 0, loc),
			want: "6月21日 10:31",
		},
		{
			name: "past same day",
			at:   now.Add(-time.Minute),
			want: "19:59",
		},
		{
			name: "cross year",
			at:   time.Date(2027, 1, 2, 8, 0, 0, 0, loc),
			want: "2027-01-02 08:00",
		},
		{
			name: "zero",
			at:   time.Time{},
			want: "—",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := formatResetAbsolute(c.at, now); got != c.want {
				t.Fatalf("got %q want %q", got, c.want)
			}
		})
	}
}
