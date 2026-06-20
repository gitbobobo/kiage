package app

import (
	"context"
	"crypto/sha256"
	"os"
	"strconv"

	"github.com/godbobo/kiage/internal/display"
	"github.com/godbobo/kiage/internal/input"
	"github.com/godbobo/kiage/internal/log"
)

func (a *App) RunKindle(ctx context.Context) error {
	log.Info("kindle loop start")
	keepScreenAwake()
	defer releaseScreenAwake()

	fbBin := os.Getenv("KIAGE_FBINK")
	display.LogFBInkProbe()
	a.mu.RLock()
	orient := a.view.Orientation
	a.mu.RUnlock()
	fb := display.New(fbBin)

	var touchQuirk input.TouchQuirk
	if vp, err := display.QueryViewport(fb.Bin); err == nil {
		a.SetScreenSize(vp.Width, vp.Height)
		fb.SetViewport(vp)
		q := vp.TouchQuirkForInput()
		touchQuirk = input.TouchQuirk{SwapAxes: q.SwapAxes, MirrorX: q.MirrorX, MirrorY: q.MirrorY}
		log.Info("fbink viewport %dx%d touch panel swap=%v mx=%v my=%v applied swap=%v mx=%v my=%v rota=%d",
			vp.Width, vp.Height,
			vp.TouchSwapAxes, vp.TouchMirrorX, vp.TouchMirrorY,
			q.SwapAxes, q.MirrorX, q.MirrorY, vp.CurrentRota)
	} else {
		log.Warn("fbink viewport query failed: %v", err)
	}
	size := a.frameSize()
	log.Info("fbink using bin=%q screen=%dx%d orient=%s", fb.Bin, size.Width, size.Height, orient)

	touch, err := input.OpenTouchListener()
	if err != nil {
		log.Warn("touch input unavailable: %v", err)
	} else if touch != nil {
		log.Info("touch input opened")
		defer touch.Close()
		go func() {
			size := a.frameSize()
			log.Info("touch listener running screen=%dx%d", size.Width, size.Height)
			touch.Run(ctx, input.ScreenMapping{
				Width:  size.Width,
				Height: size.Height,
				Quirk:  touchQuirk,
			}, &kindleTouchHandler{app: a})
			log.Info("touch listener stopped")
		}()
	}

	fullEvery := display.DefaultFullRefreshEvery
	if v, err := strconv.Atoi(os.Getenv("KIAGE_FULL_REFRESH_EVERY")); err == nil && v > 0 {
		fullEvery = v
	}

	var (
		lastHash      [32]byte
		firstDone     bool
		partialCount  int
		fbWarned      bool
		flushCount    int
	)

	pushDisplay := func() {
		if err := a.LastError(); err != nil {
			log.Error("render frame: %v", err)
			return
		}
		png := a.PNG()
		if len(png) == 0 {
			log.Warn("render frame: empty png")
			return
		}
		hash := sha256.Sum256(png)
		if firstDone && hash == lastHash {
			return
		}
		lastHash = hash

		path, err := display.WriteTempPNG(a.roots.Cache, png)
		if err != nil {
			log.Error("write frame.png: %v", err)
			return
		}
		mode, nextPartial := display.PickRefreshMode(firstDone, partialCount, fullEvery)
		if err := fb.ShowPNG(path, mode); err != nil {
			if !fbWarned {
				fbWarned = true
				log.Error("fbink show failed (mode=%d): %v", mode, err)
			}
			return
		}
		firstDone = true
		partialCount = nextPartial
		flushCount++
		if flushCount == 1 || flushCount%10 == 0 {
			log.Info("fbink show ok mode=%d bytes=%d count=%d partial=%d", mode, len(png), flushCount, partialCount)
		}
	}

	_ = a.DoSync(ctx)
	a.RefreshFrame()
	pushDisplay()

	go a.backgroundSync(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Info("kindle loop exit: %v", ctx.Err())
			return nil
		case <-a.exitCh:
			log.Info("kindle loop exit: user exit")
			return nil
		case <-a.displayCh:
			for {
				select {
				case <-a.displayCh:
				default:
					goto flush
				}
			}
		flush:
			pushDisplay()
		}
	}
}
