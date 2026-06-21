package render

import (
	"time"

	"github.com/godbobo/kiage/internal/provider"
)

func FormatPlanLine(providerID, plan, membershipType string, resetAt time.Time, resetDaysLeft int, now time.Time) string {
	if plan == "" {
		plan = "—"
	}
	reset := "—"
	resetLabel := "重置"
	if !resetAt.IsZero() {
		switch providerID {
		case provider.GLMID:
			resetLabel = "下次重置"
		case provider.MiniMaxID:
			resetLabel = "时段重置"
			if membershipType == "weekly" {
				resetLabel = "周重置"
			}
		default:
			reset = resetAt.Format("1月2日") + " (" + itoa(resetDaysLeft) + "天)"
		}
		if providerID == provider.GLMID || providerID == provider.MiniMaxID {
			reset = formatQuotaReset(resetAt, now)
		}
	}
	return "套餐 " + plan + " · " + resetLabel + " " + reset
}
