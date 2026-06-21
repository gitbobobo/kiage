package render

import (
	"strings"
	"time"

	"github.com/godbobo/kiage/internal/provider"
)

func FormatPlanLine(providerID, plan, membershipType string, resetAt time.Time, resetDaysLeft int, bars []provider.QuotaBar, now time.Time) string {
	if plan == "" {
		plan = "—"
	}
	if providerID == provider.KimiID {
		parts := []string{"套餐 " + plan}
		for _, bar := range bars {
			switch bar.Label {
			case provider.LabelIntervalQuota:
				if !bar.ResetAt.IsZero() {
					parts = append(parts, "时段重置 "+formatPlanResetRelative(bar.ResetAt, now))
				}
			case provider.LabelWeeklyQuota:
				if !bar.ResetAt.IsZero() {
					parts = append(parts, "周重置 "+formatPlanResetRelative(bar.ResetAt, now))
				}
			}
		}
		if len(parts) == 1 {
			parts = append(parts, "重置 —")
		}
		return strings.Join(parts, " · ")
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
