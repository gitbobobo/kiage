package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/godbobo/kiage/internal/log"
	"github.com/godbobo/kiage/internal/render"
	"github.com/godbobo/kiage/internal/store"
)

const (
	syncCauseStartup = "startup"
	syncCauseWake    = "wake"
	syncCauseRTC     = "rtc"
	syncCauseManual  = "manual-all"

	globalSyncStaleAfter = 5 * time.Minute
	batchDedupeWindow    = 30 * time.Second

	wlanWaitWake = 45 * time.Second

	rtcResumeDelay = 5 * time.Second
	rtcBatchDedupe = 50 * time.Minute
)

func (a *App) loadGlobalSyncTimes(ctx context.Context) {
	if ts, ok, err := a.store.GetGlobalState(ctx, store.GlobalKeyLastSyncAt); err == nil && ok {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			a.lastGlobalSyncAt = t
		}
	}
	if ts, ok, err := a.store.GetGlobalState(ctx, store.GlobalKeyLastBatchAt); err == nil && ok {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			a.lastBatchStartedAt = t
		}
	}
}

func (a *App) globalSyncStale() bool {
	a.syncTimeMu.RLock()
	defer a.syncTimeMu.RUnlock()
	if a.lastGlobalSyncAt.IsZero() {
		return true
	}
	return time.Since(a.lastGlobalSyncAt) > globalSyncStaleAfter
}

func (a *App) shouldStartBatch() bool {
	return a.shouldStartBatchCause("")
}

func (a *App) shouldStartBatchCause(cause string) bool {
	a.syncTimeMu.RLock()
	defer a.syncTimeMu.RUnlock()
	if a.lastBatchStartedAt.IsZero() {
		return true
	}
	window := batchDedupeWindow
	if cause == syncCauseRTC {
		window = rtcBatchDedupe
	}
	return time.Since(a.lastBatchStartedAt) > window
}

func (a *App) markBatchStarted() {
	now := time.Now().UTC()
	a.syncTimeMu.Lock()
	a.lastBatchStartedAt = now
	a.syncTimeMu.Unlock()
	_ = a.store.SetGlobalState(context.Background(), store.GlobalKeyLastBatchAt, now.Format(time.RFC3339))
}

func (a *App) markGlobalSyncDone() {
	now := time.Now().UTC()
	a.syncTimeMu.Lock()
	a.lastGlobalSyncAt = now
	a.syncTimeMu.Unlock()
	_ = a.store.SetGlobalState(context.Background(), store.GlobalKeyLastSyncAt, now.Format(time.RFC3339))
}

func (a *App) syncAllProvidersBatch(ctx context.Context, cause string) error {
	if !a.shouldStartBatchCause(cause) {
		log.Info("sync batch skipped dedupe cause=%s", cause)
		return nil
	}
	if !a.batchSyncing.CompareAndSwap(false, true) {
		log.Info("sync batch skipped busy cause=%s", cause)
		return nil
	}
	defer a.batchSyncing.Store(false)

	a.markBatchStarted()
	log.Info("sync batch begin cause=%s", cause)

	if cause == syncCauseWake || cause == syncCauseRTC || cause == syncCauseStartup {
		networkOK := true
		if cause == syncCauseRTC {
			rtcKeepAwake(true)
			defer rtcKeepAwake(false)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(rtcResumeDelay):
			}
			wlanEnsureOnAfterResume()
			defer wlanEnsureOff()
			networkOK = wlanConnectAfterResume(ctx)
		} else {
			wlanEnsureOn()
			networkOK = waitForWLAN(ctx, wlanWaitWake)
		}
		if !networkOK {
			log.Warn("sync batch skipped no network cause=%s", cause)
			return fmt.Errorf("network unavailable")
		}
	}

	ids := configuredProviderIDs(a)
	if len(ids) == 0 {
		return nil
	}

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)
	for i, id := range ids {
		wg.Add(1)
		go func(id string, idx int) {
			defer wg.Done()
			if cause == syncCauseWake || cause == syncCauseRTC {
				time.Sleep(time.Duration(idx) * 500 * time.Millisecond)
			}
			if err := a.runProviderSync(ctx, id); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("%s: %w", id, err))
				mu.Unlock()
			}
		}(id, i)
	}
	wg.Wait()

	if len(errs) >= len(ids) {
		log.Info("sync batch skip display refresh no data updated errors=%d", len(errs))
		log.Info("sync batch done cause=%s errors=%d", cause, len(errs))
		return errors.Join(errs...)
	}

	a.invalidateAllFrameCaches()
	if len(errs) == 0 && ctx.Err() == nil {
		a.markGlobalSyncDone()
	}

	a.refreshAfterSyncBatch(cause)

	log.Info("sync batch done cause=%s errors=%d", cause, len(errs))
	return errors.Join(errs...)
}

func (a *App) refreshAfterSyncBatch(cause string) {
	if !render.KindleUI() {
		a.refreshFrameOpts(false, false, false)
		return
	}
	forceFull := cause == syncCauseWake || cause == syncCauseRTC
	if cause == syncCauseRTC {
		time.Sleep(350 * time.Millisecond)
		a.refreshFrameOpts(true, false, true)
		a.notifyDisplayWait(true, true, 10*time.Second)
		return
	}
	a.refreshAfterBatch(forceFull)
}

func configuredProviderIDs(a *App) []string {
	var ids []string
	for _, id := range allProviderIDs() {
		if a.providerConfigured(id) {
			ids = append(ids, id)
		}
	}
	return ids
}

func (a *App) runProviderSync(ctx context.Context, id string) error {
	a.mu.Lock()
	if a.syncing[id] {
		a.mu.Unlock()
		return nil
	}
	a.syncing[id] = true
	a.mu.Unlock()
	defer a.finishSyncProvider(id)

	svc := a.syncerFor(id)
	if svc == nil {
		return fmt.Errorf("unknown provider %s", id)
	}
	err := svc.Run(ctx, "auto")
	a.mu.Lock()
	a.lastErrs[id] = err
	if id == a.activeProviderIDLocked() && err == nil {
		a.view.SyncStatus = "就绪"
	}
	a.mu.Unlock()
	if err != nil {
		log.Warn("provider sync failed id=%s err=%v", id, err)
	}
	return err
}

func (a *App) finishSyncProvider(id string) {
	a.mu.Lock()
	a.syncing[id] = false
	delete(a.progress, id)
	if snap, ok := a.frameSnaps[id]; ok {
		snap.dashValid = false
		snap.fullValid = false
		a.frameSnaps[id] = snap
	}
	active := a.activeProviderIDLocked()
	if id == active && a.view.Screen == render.ScreenProvider {
		if a.lastErrs[id] == nil {
			a.view.SyncStatus = "就绪"
		} else {
			a.view.SyncStatus = "错误"
		}
	}
	a.mu.Unlock()
}

func (a *App) invalidateAllFrameCaches() {
	a.mu.Lock()
	for id, snap := range a.frameSnaps {
		snap.dashValid = false
		snap.fullValid = false
		a.frameSnaps[id] = snap
	}
	a.invalidateFrameBaseLocked()
	a.mu.Unlock()
}

func (a *App) refreshAfterBatch(forceFull bool) {
	if render.KindleUI() {
		go func() {
			time.Sleep(350 * time.Millisecond)
			a.refreshFrameOpts(false, false, forceFull)
		}()
		return
	}
	a.refreshFrameOpts(false, false, forceFull)
}

func (a *App) refreshAfterSingleSync(id string) {
	a.mu.RLock()
	viewScreen := a.view.Screen
	active := a.activeProviderIDLocked()
	a.mu.RUnlock()

	shouldRefresh := viewScreen == render.ScreenSummary ||
		(id == active && viewScreen == render.ScreenProvider)
	if !shouldRefresh {
		return
	}
	if render.KindleUI() {
		go func() {
			time.Sleep(350 * time.Millisecond)
			a.refreshFrame(false)
		}()
		return
	}
	a.RefreshFrame()
}

func (a *App) devBackgroundSync(ctx context.Context) {
	if render.KindleUI() {
		return
	}
	_ = a.syncAllProvidersBatch(ctx, syncCauseManual)
	a.mu.RLock()
	interval := time.Duration(a.cfg.RefreshIntervalSec) * time.Second
	a.mu.RUnlock()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = a.syncAllProvidersBatch(ctx, syncCauseManual)
			a.mu.RLock()
			next := time.Duration(a.cfg.RefreshIntervalSec) * time.Second
			a.mu.RUnlock()
			if next != interval {
				interval = next
				ticker.Reset(interval)
			}
		}
	}
}
