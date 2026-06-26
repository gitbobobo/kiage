package render

import (
	"strings"
	"time"

	"github.com/godbobo/kiage/internal/provider"
)

func FormatPlanLine(providerID, plan, membershipType string, resetAt time.Time, bars []provider.QuotaBar, now time.Time) string {
	if plan == "" {
		plan = "—"
	}
	if providerID == provider.KimiID {
		parts := []string{"套餐 " + plan}
		for _, bar := range bars {
			switch bar.Label {
			case provider.LabelIntervalQuota:
				if !bar.ResetAt.IsZero() {
					parts = append(parts, "时段重置 "+formatResetAbsolute(bar.ResetAt, now))
				}
			case provider.LabelWeeklyQuota:
				if !bar.ResetAt.IsZero() {
					parts = append(parts, "周重置 "+formatResetAbsolute(bar.ResetAt, now))
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
			resetLabel = "重置"
		}
		reset = formatResetAbsolute(resetAt, now)
	}
	return "套餐 " + plan + " · " + resetLabel + " " + reset
}
