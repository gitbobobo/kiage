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

func (h *kindleTouchHandler) OnTap(x, y int) {
	h.app.handleTopTap(x, y)
}

func (a *App) handleTopTap(x, y int) {
	a.mu.Lock()
	if time.Since(a.lastTouchTap) < kindleTouchDebounce {
		a.mu.Unlock()
		log.Info("touch tap (%d,%d) ignored debounce", x, y)
		return
	}
	a.lastTouchTap = time.Now()
	name := a.prov.DisplayName()
	a.mu.Unlock()

	size := a.frameSize()
	regions := render.TopControlsHitRegions(size, name, a.View().ChartMetric)

	action := ""
	if regions.ProviderTitle.ContainsPadAsymmetric(x, y, 12, 12, 12, 48) {
		action = "sync"
	} else {
		action = render.KindleTopBarAction(size, x, y, regions)
	}

	if action == "" {
		log.Info("touch tap (%d,%d) no hit bar_y<120 metric_x=%d-%d settings_x=%d-%d exit_x=%d-%d",
			x, y,
			regions.MetricToggle.X-16, regions.MetricToggle.X+regions.MetricToggle.W+16,
			regions.Settings.X-16, regions.Settings.X+regions.Settings.W+16,
			regions.Exit.X-16, regions.Exit.X+regions.Exit.W+16)
		return
	}

	log.Info("touch tap (%d,%d) action=%s", x, y, action)
	switch action {
	case "sync":
		a.startSyncAsync(context.Background())
	case "metric_toggle":
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
