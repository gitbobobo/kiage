package app

import (
	"context"
	"crypto/sha256"
	"os"
	"strconv"
	"time"

	"github.com/godbobo/kiage/internal/display"
	"github.com/godbobo/kiage/internal/input"
	"github.com/godbobo/kiage/internal/log"
)

func (a *App) RunKindle(ctx context.Context) error {
	log.Info("kindle loop start")
	keepScreenAwake()
	defer releaseScreenAwake()

	fbBin := os.Getenv("KIAGE_FBINK")
	if fbBin == "" {
		display.LogFBInkProbe()
	}
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
		lastHash     [32]byte
		firstDone    bool
		partialCount int
		flushCount   int
	)

	pushDisplay := func(urgent bool) {
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
		if firstDone && hash == lastHash && !urgent {
			return
		}
		lastHash = hash

		path, err := display.WriteTempPNG(a.roots.Cache, png)
		if err != nil {
			log.Error("write frame.png: %v", err)
			return
		}

		var mode display.RefreshMode
		nextPartial := partialCount
		switch {
		case !firstDone:
			mode = display.RefreshFirst
		case urgent:
			// 交互用 GL16 局部刷（真机 DU 不稳定）
			mode = display.RefreshPartial
		default:
			mode, nextPartial = display.PickRefreshMode(firstDone, partialCount, fullEvery)
		}

		fbStart := time.Now()
		if err := fb.ShowPNG(path, mode); err != nil {
			log.Error("fbink show failed mode=%d urgent=%v: %v", mode, urgent, err)
			if urgent && mode == display.RefreshPartial {
				log.Info("fbink retry urgent as full GC16")
				if err2 := fb.ShowPNG(path, display.RefreshFull); err2 != nil {
					log.Error("fbink retry failed: %v", err2)
					return
				}
			} else {
				return
			}
		}
		firstDone = true
		if !urgent {
			partialCount = nextPartial
		}
		flushCount++
		log.Info("fbink show ok mode=%d bytes=%d count=%d partial=%d urgent=%v fb_ms=%d",
			mode, len(png), flushCount, partialCount, urgent, time.Since(fbStart).Milliseconds())
	}

	// 刷屏专用协程：pushDisplay 可能阻塞数秒，不可占主循环
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case n := <-a.displayCh:
				urgent := n.urgent
			drain:
				for {
					select {
					case n2 := <-a.displayCh:
						if n2.urgent {
							urgent = true
						}
					default:
						break drain
					}
				}
				pushDisplay(urgent)
			}
		}
	}()

	paintStart := time.Now()
	a.RefreshFrame()
	log.Info("kindle first paint render ms=%d", time.Since(paintStart).Milliseconds())
	go a.backgroundSync(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Info("kindle loop exit: %v", ctx.Err())
			return nil
		case <-a.exitCh:
			log.Info("kindle loop exit: user exit")
			return nil
		}
	}
}
