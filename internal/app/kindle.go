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

	initialRota := input.QueryInitialRota(fb.Bin)
	a.initPortraitRota(initialRota)

	if vp, err := queryViewportForRota(fb.Bin, initialRota); err == nil {
		fb.SetViewport(vp)
		a.storeTouchMappingForRota(vp, initialRota)
		q := vp.TouchQuirkForRota(initialRota)
		log.Info("fbink viewport %dx%d touch panel swap=%v mx=%v my=%v applied swap=%v mx=%v my=%v rota=%d initial=%d",
			vp.Width, vp.Height,
			vp.TouchSwapAxes, vp.TouchMirrorX, vp.TouchMirrorY,
			q.SwapAxes, q.MirrorX, q.MirrorY, vp.CurrentRota, initialRota)
	} else {
		log.Warn("fbink viewport query failed: %v", err)
	}

	size := a.frameSize()
	log.Info("fbink using bin=%q screen=%dx%d orient=%s portrait_rota=%d",
		fb.Bin, size.Width, size.Height, orient, a.currentPortraitRota())

	touch, err := input.OpenTouchListener()
	if err != nil {
		log.Warn("touch input unavailable: %v", err)
	} else if touch != nil {
		log.Info("touch input opened")
		defer touch.Close()
		go func() {
			size := a.frameSize()
			log.Info("touch listener running screen=%dx%d", size.Width, size.Height)
			touch.Run(ctx, a.touchScreenFn(), &kindleTouchHandler{app: a})
			log.Info("touch listener stopped")
		}()
	}

	if os.Getenv("KIAGE_ORIENTATION") == "" {
		orientListener, err := input.OpenOrientationListener()
		if err != nil {
			log.Warn("orientation listener unavailable: %v", err)
		} else if orientListener != nil {
			log.Info("orientation listener opened")
			defer orientListener.Close()
			go orientListener.Run(ctx, func(rota int) {
				a.applyRotation(ctx, fb, rota)
			})
		} else {
			log.Info("orientation listener not found (non-Oasis or no accel device)")
		}
	} else {
		log.Info("orientation listener disabled (KIAGE_ORIENTATION set)")
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

	pushDisplay := func(urgent, forceFull bool) {
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
		if firstDone && hash == lastHash && !urgent && !forceFull {
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
		case forceFull:
			mode = display.RefreshFull
			partialCount = 0
		case urgent:
			mode = display.RefreshPartial
		default:
			mode, nextPartial = display.PickRefreshMode(firstDone, partialCount, fullEvery)
		}

		fbStart := time.Now()
		if err := fb.ShowPNG(path, mode); err != nil {
			log.Error("fbink show failed mode=%d urgent=%v forceFull=%v: %v", mode, urgent, forceFull, err)
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
		if forceFull {
			partialCount = 0
		} else if !urgent {
			partialCount = nextPartial
		}
		firstDone = true
		flushCount++
		log.Info("fbink show ok mode=%d bytes=%d count=%d partial=%d urgent=%v forceFull=%v portrait_rota=%d fb_ms=%d",
			mode, len(png), flushCount, partialCount, urgent, forceFull, a.currentPortraitRota(), time.Since(fbStart).Milliseconds())
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case n := <-a.displayCh:
				urgent := n.urgent
				forceFull := n.forceFull
			drain:
				for {
					select {
					case n2 := <-a.displayCh:
						if n2.urgent {
							urgent = true
						}
						if n2.forceFull {
							forceFull = true
						}
					default:
						break drain
					}
				}
				pushDisplay(urgent, forceFull)
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
