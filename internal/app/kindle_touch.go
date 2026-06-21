package app

import (
	"context"
	"os/exec"
	"time"

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

func (h *kindleKeyHandler) OnScreenUp() {
	go h.app.toggleProvider()
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
	metric := a.view.ChartMetric
	supportsCost := a.view.SupportsCost
	a.mu.Unlock()

	size := a.frameSize()
	regions := render.TopControlsHitRegions(size, name, metric)

	action := ""
	if regions.ProviderTitle.ContainsPadAsymmetric(x, y, 12, 12, 12, 48) {
		action = "sync"
	} else {
		action = render.KindleTopControlAction(x, y, regions)
	}

	if action == "" {
		return
	}

	log.Info("touch tap (%d,%d) action=%s", x, y, action)
	switch action {
	case "sync":
		a.startSyncProviderAsync(context.Background(), providerID)
	case "metric_toggle":
		if !supportsCost {
			return
		}
		a.SetViewUrgent(func(v *render.ViewState) {
			if v.ChartMetric == "token" {
				v.ChartMetric = "cost"
			} else {
				v.ChartMetric = "token"
			}
		})
	case "settings":
		go func() {
			if err := a.ToggleSettingsServer(); err != nil {
				log.Warn("toggle settings: %v", err)
			}
		}()
	case "exit":
		a.requestExit()
	}
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
