package render

import (
	"github.com/godbobo/kiage/internal/aggregate"
	"github.com/godbobo/kiage/internal/provider"
)

// BarsForProvider 返回应绘制的配额条；未配置或无数据时返回 nil。
func BarsForProvider(providerID string, dash aggregate.Dashboard, configured bool) []provider.QuotaBar {
	if !configured {
		return nil
	}
	if len(dash.Bars) > 0 {
		return dash.Bars
	}
	if providerID == provider.CursorID {
		return provider.CursorBarsFromPercents(dash.TotalPercent, dash.ComposerPercent, dash.APIPercent)
	}
	return nil
}
