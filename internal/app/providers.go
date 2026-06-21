package app

import (
	"context"

	"slices"

	"github.com/godbobo/kiage/internal/config"
	"github.com/godbobo/kiage/internal/provider"
	"github.com/godbobo/kiage/internal/provider/cursor"
	"github.com/godbobo/kiage/internal/provider/glm"
	"github.com/godbobo/kiage/internal/provider/kimi"
	"github.com/godbobo/kiage/internal/provider/minimax"
	"github.com/godbobo/kiage/internal/render"
	syncer "github.com/godbobo/kiage/internal/sync"
)

func allProviderIDs() []string {
	return []string{provider.CursorID, provider.GLMID, provider.MiniMaxID, provider.KimiID}
}

func detailProviderIDs() []string {
	return []string{provider.CursorID, provider.GLMID}
}

func isDetailProvider(id string) bool {
	return slices.Contains(detailProviderIDs(), id)
}

func buildProviders(cfg config.Config) (map[string]provider.Provider, error) {
	out := make(map[string]provider.Provider, 4)
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
	minimaxProv, err := minimax.New(cfg)
	if err != nil {
		return nil, err
	}
	out[provider.MiniMaxID] = minimaxProv
	kimiProv, err := kimi.New(cfg)
	if err != nil {
		return nil, err
	}
	out[provider.KimiID] = kimiProv
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
	case provider.MiniMaxID:
		return cfg.MiniMax.APIKey != ""
	case provider.KimiID:
		return cfg.Kimi.APIKey != ""
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
	if !ok || !isDetailProvider(id) {
		id = provider.CursorID
	}
	a.activeProviderID = id
}

func (a *App) setActiveProvider(id string) {
	if !isDetailProvider(id) {
		return
	}
	a.setScreen(render.ScreenProvider, id, true)
}

func (a *App) providerConfiguredLocked(id string) bool {
	switch id {
	case provider.CursorID:
		return a.cfg.Cursor.SessionToken != ""
	case provider.GLMID:
		return a.cfg.GLM.APIKey != ""
	case provider.MiniMaxID:
		return a.cfg.MiniMax.APIKey != ""
	case provider.KimiID:
		return a.cfg.Kimi.APIKey != ""
	default:
		return false
	}
}

func (a *App) attachSyncProgress(id string, svc *syncer.Service) {
	svc.OnProgress(func(p syncer.Progress) {
		a.mu.Lock()
		a.progress[id] = p.Message
		if id == a.activeProviderIDLocked() && a.view.Screen == render.ScreenProvider {
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
