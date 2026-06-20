package app

import (
	"context"
	"os/exec"

	"github.com/godbobo/kiage/internal/log"
	"github.com/godbobo/kiage/internal/render"
)

type kindleTouchHandler struct {
	app *App
}

func (h *kindleTouchHandler) OnTap(x, y int) {
	h.app.handleTopTap(x, y)
}

func (a *App) handleTopTap(x, y int) {
	a.mu.RLock()
	name := a.prov.DisplayName()
	a.mu.RUnlock()

	size := a.frameSize()
	regions := render.TopControlsHitRegions(size, name)

	action := ""
	switch {
	case regions.ProviderTitle.Contains(x, y):
		action = "sync"
	case regions.Exit.Contains(x, y):
		action = "exit"
	case regions.Settings.Contains(x, y):
		action = "settings"
	case regions.MetricToggle.Contains(x, y):
		action = "metric_toggle"
	default:
		action = render.HitTopRightBar(size, x, y)
	}

	if action == "" {
		log.Info("touch tap (%d,%d) no hit", x, y)
		return
	}

	log.Info("touch tap (%d,%d) action=%s", x, y, action)
	switch action {
	case "sync":
		a.startSyncAsync(context.Background())
	case "metric_toggle":
		a.SetView(func(v *render.ViewState) {
			if v.ChartMetric == "token" {
				v.ChartMetric = "cost"
			} else {
				v.ChartMetric = "token"
			}
		})
	case "settings":
		_ = a.ToggleSettingsServer()
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
