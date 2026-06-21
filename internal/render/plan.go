package render

import (
	"time"

	"github.com/godbobo/kiage/internal/provider"
)

func FormatPlanLine(providerID, plan string, resetAt time.Time, resetDaysLeft int, now time.Time) string {
	if plan == "" {
		plan = "—"
	}
	reset := "—"
	resetLabel := "重置"
	if !resetAt.IsZero() {
		if providerID == provider.GLMID {
			resetLabel = "下次重置"
			reset = formatQuotaReset(resetAt, now)
		} else {
			reset = resetAt.Format("1月2日") + " (" + itoa(resetDaysLeft) + "天)"
		}
	}
	return "套餐 " + plan + " · " + resetLabel + " " + reset
}
