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
	if action == input.ScreenDownDouble {
		h.app.clearInputSuppress()
		h.app.requestExit()
		return
	}
	if h.app.inputSuppressed() {
		log.Info("key ignored input suppress")
		return
	}
	switch action {
	case input.ScreenUpSingle:
		go h.app.cycleScreen()
	case input.ScreenUpDouble:
		h.app.mu.RLock()
		screen := h.app.view.Screen
		supportsCost := h.app.view.SupportsCost
		h.app.mu.RUnlock()
		if screen != render.ScreenProvider || !supportsCost {
			return
		}
		h.app.SetViewUrgent(func(v *render.ViewState) {
			if v.ChartMetric == render.MetricToken {
				v.ChartMetric = render.MetricCost
			} else {
				v.ChartMetric = render.MetricToken
			}
		})
	case input.ScreenDownSingle:
		go func() {
			if err := h.app.ToggleSettingsServer(); err != nil {
				log.Warn("toggle settings: %v", err)
			}
		}()
	}
}

func (a *App) handleTopTap(x, y int) {
	if a.inputSuppressed() {
		log.Info("touch tap (%d,%d) ignored input suppress", x, y)
		return
	}
	a.mu.Lock()
	if a.view.Screen == render.ScreenSummary {
		a.mu.Unlock()
		return
	}
	if time.Since(a.lastTouchTap) < kindleTouchDebounce {
		a.mu.Unlock()
		log.Info("touch tap (%d,%d) ignored debounce", x, y)
		return
	}
	a.lastTouchTap = time.Now()
	providerID := a.activeProviderIDLocked()
	name := a.view.ProviderName
	a.mu.Unlock()

	regions := render.TopControlsHitRegions(render.ScreenProvider, name)

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

func (a *App) shutdownKindleInputs() {
	_ = exec.Command("sh", "-c", `[ -e /proc/keypad ] && echo unlock >/proc/keypad; [ -e /proc/fiveway ] && echo unlock >/proc/fiveway`).Run()
}
