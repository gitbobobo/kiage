package app

import (
	"context"

	"github.com/godbobo/kiage/internal/config"
	"github.com/godbobo/kiage/internal/provider"
	"github.com/godbobo/kiage/internal/provider/cursor"
	"github.com/godbobo/kiage/internal/provider/glm"
	syncer "github.com/godbobo/kiage/internal/sync"
)

func allProviderIDs() []string {
	return []string{provider.CursorID, provider.GLMID}
}

func buildProviders(cfg config.Config) (map[string]provider.Provider, error) {
	out := make(map[string]provider.Provider, 2)
	cursorProv, err := cursor.New(cfg)
	if err != nil {
		return nil, err
	}
	out[provider.CursorID] = cursorProv
	glmProv, err := glm.New(cfg)
	if err != nil {
		return nil, err
	}
	out[provider.GLMID] = glmProv
	return out, nil
}

func (a *App) providerConfigured(id string) bool {
	a.mu.RLock()
	cfg := a.cfg
	a.mu.RUnlock()
	switch id {
	case provider.CursorID:
		return cfg.Cursor.SessionToken != ""
	case provider.GLMID:
		return cfg.GLM.APIKey != ""
	default:
		return false
	}
}

func (a *App) activeProviderIDLocked() string {
	if a.activeProviderID == "" {
		return provider.CursorID
	}
	return a.activeProviderID
}

func (a *App) loadActiveProvider(ctx context.Context) {
	id, ok, _ := a.store.GetState(ctx, provider.AppStateProvider, "active_provider")
	if !ok || (id != provider.CursorID && id != provider.GLMID) {
		id = provider.CursorID
	}
	a.activeProviderID = id
}

func (a *App) setActiveProvider(id string) {
	if id != provider.CursorID && id != provider.GLMID {
		return
	}
	a.mu.Lock()
	if a.activeProviderID == id {
		a.mu.Unlock()
		return
	}
	a.activeProviderID = id
	if prov, ok := a.providers[id]; ok {
		a.view.ProviderID = id
		a.view.ProviderName = prov.DisplayName()
		caps := prov.Capabilities()
		a.view.SupportsCost = caps.SupportsCost
		if !caps.SupportsCost {
			a.view.ChartMetric = "token"
		}
		if !a.providerConfiguredLocked(id) {
			a.view.SyncStatus = "未配置 API Key"
		} else if a.lastErrs[id] != nil {
			a.view.SyncStatus = "错误"
		} else {
			a.view.SyncStatus = "就绪"
		}
	}
	a.mu.Unlock()

	_ = a.store.SetState(context.Background(), provider.AppStateProvider, "active_provider", id)
	a.refreshFrameViewOnly(true)
	if a.providerConfigured(id) {
		a.startSyncProviderAsync(context.Background(), id)
	}
}

func (a *App) providerConfiguredLocked(id string) bool {
	switch id {
	case provider.CursorID:
		return a.cfg.Cursor.SessionToken != ""
	case provider.GLMID:
		return a.cfg.GLM.APIKey != ""
	default:
		return false
	}
}

func (a *App) toggleProvider() {
	a.mu.RLock()
	cur := a.activeProviderID
	a.mu.RUnlock()
	if cur == provider.CursorID {
		a.setActiveProvider(provider.GLMID)
	} else {
		a.setActiveProvider(provider.CursorID)
	}
}

func (a *App) attachSyncProgress(id string, svc *syncer.Service) {
	svc.OnProgress(func(p syncer.Progress) {
		a.mu.Lock()
		a.progress[id] = p.Message
		if id == a.activeProviderIDLocked() {
			a.view.SyncMessage = p.Message
		}
		a.mu.Unlock()
	})
}

func (a *App) reloadProviders(cfg config.Config, providers map[string]provider.Provider) error {
	var err error
	if providers == nil {
		providers, err = buildProviders(cfg)
		if err != nil {
			return err
		}
	}
	syncers := make(map[string]*syncer.Service, len(providers))
	for id, prov := range providers {
		svc := syncer.New(prov, a.store)
		a.attachSyncProgress(id, svc)
		syncers[id] = svc
	}

	a.mu.Lock()
	a.cfg = cfg
	a.providers = providers
	a.syncers = syncers
	if prov, ok := providers[a.activeProviderID]; ok {
		a.view.ProviderName = prov.DisplayName()
		a.view.SupportsCost = prov.Capabilities().SupportsCost
	}
	a.mu.Unlock()
	return nil
}
