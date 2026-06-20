package render

import (
	"fmt"
	"time"
)

func formatQuotaReset(at, now time.Time) string {
	if at.IsZero() {
		return "—"
	}
	loc := now.Location()
	at = at.In(loc)
	now = now.In(loc)
	d := at.Sub(now)
	if d < time.Minute {
		return "即将重置"
	}
	if d < time.Hour {
		mins := int(d / time.Minute)
		return fmt.Sprintf("约 %dm 后", mins)
	}
	if at.Year() == now.Year() && at.YearDay() == now.YearDay() {
		return at.Format("15:04")
	}
	if at.Year() == now.Year() {
		return at.Format("1月2日 15:04")
	}
	return at.Format("2006-01-02 15:04")
}
