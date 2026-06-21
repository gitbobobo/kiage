package app

import (
	"context"

	"github.com/godbobo/kiage/internal/display"
	"github.com/godbobo/kiage/internal/input"
	"github.com/godbobo/kiage/internal/log"
	"github.com/godbobo/kiage/internal/render"
)

func (a *App) initPortraitRota(rota int, fbRota int) {
	if rota != 2 {
		rota = 0
	}
	a.portraitRota.Store(int32(rota))
	a.baselineRota.Store(int32(rota))
	a.fbRota.Store(int32(fbRota))
	a.mu.Lock()
	a.view.PortraitRota = rota
	a.mu.Unlock()
}

func (a *App) currentPortraitRota() int {
	return int(a.portraitRota.Load())
}

func (a *App) displayPortraitRota() int {
	input := a.currentPortraitRota()
	baseline := int(a.baselineRota.Load())
	return render.PortraitRotaForDisplay(input, 0, baseline)
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

// queryViewportForRota 获取 fbink 视口；FBINK_NO_SW_ROTA 下 fb currentRota 可能与物理朝向不一致。
func queryViewportForRota(bin string, _ int) (display.Viewport, error) {
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
		log.Info("orientation fb_rota=%d accel=%d", vp.CurrentRota, wantRota)
	}

	old := a.currentPortraitRota()
	fb.SetViewport(vp)
	a.fbRota.Store(int32(vp.CurrentRota))
	a.storeTouchMappingForRota(vp, wantRota)
	a.portraitRota.Store(int32(wantRota))
	a.mu.Lock()
	a.view.PortraitRota = wantRota
	a.frameBase = nil
	a.mu.Unlock()

	q := vp.TouchQuirkForRota(wantRota)
	log.Info("orientation input %d->%d viewport=%dx%d fb_rota=%d display_rota=%d baseline=%d quirk=%+v",
		old, wantRota, vp.Width, vp.Height, vp.CurrentRota, a.displayPortraitRota(), int(a.baselineRota.Load()), q)
	a.refreshFrameOpts(true, false, true)
}
