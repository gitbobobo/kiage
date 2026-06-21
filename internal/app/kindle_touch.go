package app

import (
	"context"
	"os/exec"
	"time"

	"github.com/godbobo/kiage/internal/input"
	"github.com/godbobo/kiage/internal/log"
	"github.com/godbobo/kiage/internal/render"
)

const kindleTouchDebounce = 300 * time.Millisecond

type kindleTouchHandler struct {
	app *App
}

func (h *kindleTouchHandler) TouchQuirkVersion() uint64 {
	return h.app.touchQuirkVer.Load()
}

func (h *kindleTouchHandler) OnTap(x, y int) {
	h.app.handleTopTap(x, y)
}

type kindleKeyHandler struct {
	app *App
}

func (h *kindleKeyHandler) PortraitRota() int {
	return h.app.currentPortraitRota()
}

func (h *kindleKeyHandler) OnScreenKey(action input.ScreenKeyAction) {
	switch action {
	case input.ScreenUpSingle:
		go h.app.toggleProvider()
	case input.ScreenUpDouble:
		h.app.mu.RLock()
		supportsCost := h.app.view.SupportsCost
		h.app.mu.RUnlock()
		if !supportsCost {
			return
		}
		h.app.SetViewUrgent(func(v *render.ViewState) {
			if v.ChartMetric == "token" {
				v.ChartMetric = "cost"
			} else {
				v.ChartMetric = "token"
			}
		})
	case input.ScreenDownSingle:
		go func() {
			if err := h.app.ToggleSettingsServer(); err != nil {
				log.Warn("toggle settings: %v", err)
			}
		}()
	case input.ScreenDownDouble:
		h.app.requestExit()
	}
}

func (a *App) handleTopTap(x, y int) {
	a.mu.Lock()
	if time.Since(a.lastTouchTap) < kindleTouchDebounce {
		a.mu.Unlock()
		log.Info("touch tap (%d,%d) ignored debounce", x, y)
		return
	}
	a.lastTouchTap = time.Now()
	providerID := a.activeProviderIDLocked()
	name := a.view.ProviderName
	a.mu.Unlock()

	regions := render.TopControlsHitRegions(name)

	if !regions.ProviderTitle.ContainsPadAsymmetric(x, y, 12, 12, 12, 48) {
		return
	}

	log.Info("touch tap (%d,%d) action=sync", x, y)
	a.startSyncProviderAsync(context.Background(), providerID)
}

func (a *App) requestExit() {
	select {
	case a.exitCh <- struct{}{}:
	default:
	}
}

func keepScreenAwake() {
	_ = exec.Command("lipc-set-prop", "com.lab126.powerd", "preventScreenSaver", "1").Run()
}

func releaseScreenAwake() {
	_ = exec.Command("lipc-set-prop", "com.lab126.powerd", "preventScreenSaver", "0").Run()
}
