package app

import (
	"context"

	"github.com/godbobo/kiage/internal/aggregate"
	"github.com/godbobo/kiage/internal/provider"
	"github.com/godbobo/kiage/internal/render"
)

func frameBaseKey(screen render.Screen, providerID string) string {
	if screen == render.ScreenSummary {
		return "summary"
	}
	return "provider:" + providerID
}

func nextScreen(screen render.Screen, providerID string) (render.Screen, string) {
	ids := allProviderIDs()
	switch screen {
	case render.ScreenSummary:
		if len(ids) == 0 {
			return render.ScreenSummary, ""
		}
		return render.ScreenProvider, ids[0]
	case render.ScreenProvider:
		idx := 0
		for i, id := range ids {
			if id == providerID {
				idx = i
				break
			}
		}
		if idx+1 < len(ids) {
			return render.ScreenProvider, ids[idx+1]
		}
		return render.ScreenSummary, ""
	default:
		return render.ScreenSummary, ""
	}
}

func (a *App) providerSyncStatusForDashLocked(id string, dash aggregate.Dashboard) string {
	if !a.providerConfiguredLocked(id) {
		return "未配置 API Key"
	}
	if a.syncing[id] {
		return "同步中"
	}
	if a.lastErrs[id] != nil {
		return "错误"
	}
	if dash.LastUpdatedAt.IsZero() {
		return "尚未同步"
	}
	return "就绪"
}

func (a *App) allSummarySnapsReadyLocked() bool {
	for _, id := range allProviderIDs() {
		if !a.frameSnaps[id].dashValid {
			return false
		}
	}
	return true
}

func (a *App) setScreen(screen render.Screen, providerID string, triggerSync bool) {
	if screen == render.ScreenProvider {
		if providerID != provider.CursorID && providerID != provider.GLMID {
			return
		}
	}

	a.mu.Lock()
	a.view.Screen = screen
	if screen == render.ScreenSummary {
		a.view.ProviderID = ""
		a.view.ProviderName = ""
		a.view.SyncStatus = ""
		a.view.SyncMessage = ""
		a.invalidateFrameBaseLocked()
		a.mu.Unlock()
		a.refreshFrameViewOnly(true)
		return
	}

	a.activeProviderID = providerID
	if prov, ok := a.providers[providerID]; ok {
		a.view.ProviderID = providerID
		a.view.ProviderName = prov.DisplayName()
		caps := prov.Capabilities()
		a.view.SupportsCost = caps.SupportsCost
		if !caps.SupportsCost {
			a.view.ChartMetric = render.MetricToken
		}
		dash := a.frameSnaps[providerID].dash
		a.view.SyncStatus = a.providerSyncStatusForDashLocked(providerID, dash)
	}
	a.invalidateFrameBaseLocked()
	a.mu.Unlock()

	_ = a.store.SetState(context.Background(), provider.AppStateProvider, "active_provider", providerID)
	a.refreshFrameViewOnly(true)
	if triggerSync && a.providerConfigured(providerID) {
		a.startSyncProviderAsync(context.Background(), providerID)
	}
}

func (a *App) cycleScreen() {
	a.mu.RLock()
	screen := a.view.Screen
	providerID := a.view.ProviderID
	if screen == render.ScreenProvider && providerID == "" {
		providerID = a.activeProviderIDLocked()
	}
	a.mu.RUnlock()

	next, nextID := nextScreen(screen, providerID)
	a.setScreen(next, nextID, false)
}

type overviewBuildItem struct {
	id        string
	dash      aggregate.Dashboard
	dashValid bool
	buildErr  error
	needBuild bool
}

func (a *App) buildOverview(ctx context.Context, viewOnly bool) aggregate.Overview {
	ids := allProviderIDs()
	items := make([]overviewBuildItem, len(ids))

	a.mu.RLock()
	for i, id := range ids {
		snap := a.frameSnaps[id]
		items[i] = overviewBuildItem{
			id:        id,
			dash:      snap.dash,
			dashValid: snap.dashValid,
			needBuild: !viewOnly || !snap.dashValid,
		}
	}
	a.mu.RUnlock()

	for i := range items {
		if !items[i].needBuild {
			continue
		}
		dash, err := a.agg.Build(ctx, items[i].id)
		items[i].dash = dash
		items[i].buildErr = err
		items[i].dashValid = err == nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	var overview aggregate.Overview
	for _, item := range items {
		if item.dashValid {
			snap := a.frameSnaps[item.id]
			snap.dash = item.dash
			snap.dashValid = true
			a.frameSnaps[item.id] = snap
		}
		dash := item.dash
		if item.buildErr != nil {
			dash.SyncStatus = "错误"
		} else {
			if msg := a.progress[item.id]; msg != "" && a.syncing[item.id] {
				dash.SyncMessage = msg
			}
			dash.SyncStatus = a.providerSyncStatusForDashLocked(item.id, dash)
		}

		name := item.id
		if prov, ok := a.providers[item.id]; ok {
			name = prov.DisplayName()
		}
		overview.Providers = append(overview.Providers, aggregate.ProviderSummary{
			ProviderID:  item.id,
			DisplayName: name,
			Configured:  a.providerConfiguredLocked(item.id),
			Dashboard:   dash,
		})
	}
	return overview
}
