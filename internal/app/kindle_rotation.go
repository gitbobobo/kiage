package app

import (
	"context"
	"time"

	"github.com/godbobo/kiage/internal/display"
	"github.com/godbobo/kiage/internal/input"
	"github.com/godbobo/kiage/internal/log"
)

func (a *App) initPortraitRota(rota int) {
	if rota != 2 {
		rota = 0
	}
	a.portraitRota.Store(int32(rota))
	a.mu.Lock()
	a.view.PortraitRota = rota
	a.mu.Unlock()
}

func (a *App) currentPortraitRota() int {
	return int(a.portraitRota.Load())
}

func (a *App) markKindleReady() {
	a.kindleReady.Store(true)
}

func (a *App) storeTouchMappingForRota(vp display.Viewport, rota int) {
	q := vp.TouchQuirkForRota(rota)
	m := input.ScreenMapping{
		Width:  vp.Width,
		Height: vp.Height,
		Quirk:  input.TouchQuirk{SwapAxes: q.SwapAxes, MirrorX: q.MirrorX, MirrorY: q.MirrorY},
	}
	a.touchMapping.Store(m)
	a.touchQuirkVer.Add(1)
	a.SetScreenSize(vp.Width, vp.Height)
}

func (a *App) touchScreenFn() func() input.ScreenMapping {
	return func() input.ScreenMapping {
		if v := a.touchMapping.Load(); v != nil {
			return v.(input.ScreenMapping)
		}
		size := a.frameSize()
		return input.ScreenMapping{Width: size.Width, Height: size.Height}
	}
}

// queryViewportForRota 等待 fbink 报告目标 rota；超时仍返回最近一次有效 viewport。
func queryViewportForRota(bin string, wantRota int) (display.Viewport, error) {
	const (
		attempts = 25
		interval = 150 * time.Millisecond
	)
	var last display.Viewport
	var lastErr error
	for i := 0; i < attempts; i++ {
		if i > 0 {
			time.Sleep(interval)
		}
		vp, err := display.QueryViewport(bin)
		if err != nil {
			lastErr = err
			continue
		}
		last = vp
		if vp.CurrentRota == wantRota {
			return vp, nil
		}
	}
	if last.Width > 0 {
		return last, nil
	}
	if lastErr != nil {
		return display.Viewport{}, lastErr
	}
	return display.QueryViewport(bin)
}

func (a *App) applyRotation(_ context.Context, fb *display.FBInk, wantRota int) {
	if wantRota != 0 && wantRota != 2 {
		return
	}
	if !a.kindleReady.Load() {
		return
	}
	if a.currentPortraitRota() == wantRota {
		return
	}

	vp, err := display.QueryViewport(fb.Bin)
	if err != nil {
		log.Warn("orientation viewport query: %v", err)
		return
	}
	if vp.CurrentRota != 0 && vp.CurrentRota != 2 {
		log.Warn("orientation viewport rota=%d not portrait, skip", vp.CurrentRota)
		return
	}
	if vp.CurrentRota != wantRota {
		log.Info("orientation fb_rota=%d portrait=%d (use portrait for flip/touch)", vp.CurrentRota, wantRota)
	}

	old := a.currentPortraitRota()
	fb.SetViewport(vp)
	a.storeTouchMappingForRota(vp, wantRota)
	a.portraitRota.Store(int32(wantRota))
	a.mu.Lock()
	a.view.PortraitRota = wantRota
	a.mu.Unlock()

	q := vp.TouchQuirkForRota(wantRota)
	a.mu.RLock()
	providerID := a.activeProviderIDLocked()
	useFast := a.frameSnaps[providerID].valid && a.frameBase != nil && a.frameBaseProvider == providerID
	a.mu.RUnlock()
	log.Info("orientation applied %d->%d viewport=%dx%d fb_rota=%d quirk=%+v fast=%v",
		old, wantRota, vp.Width, vp.Height, vp.CurrentRota, q, useFast)
	if useFast {
		a.refreshFrameOpts(true, true, true)
	} else {
		a.refreshFrameOpts(true, false, true)
	}
}
