package render

import (
	"time"
)

func formatResetAbsolute(at, now time.Time) string {
	if at.IsZero() {
		return "—"
	}
	loc := now.Location()
	at = at.In(loc)
	now = now.In(loc)
	if at.Year() == now.Year() && at.YearDay() == now.YearDay() {
		return at.Format("15:04")
	}
	if at.Year() == now.Year() {
		return at.Format("1月2日 15:04")
	}
	return at.Format("2006-01-02 15:04")
}
