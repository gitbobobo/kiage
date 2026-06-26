package app

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/godbobo/kiage/internal/aggregate"
	"github.com/godbobo/kiage/internal/config"
	"github.com/godbobo/kiage/internal/log"
	"github.com/godbobo/kiage/internal/paths"
	"github.com/godbobo/kiage/internal/provider"
	"github.com/godbobo/kiage/internal/render"
	"github.com/godbobo/kiage/internal/store"
	syncer "github.com/godbobo/kiage/internal/sync"
)

type frameSnapshot struct {
	dash      aggregate.Dashboard
	line      []aggregate.LinePoint
	heat      aggregate.HeatmapStats
	dashValid bool
	fullValid bool
}

type App struct {
	roots       paths.Roots
	cfg         config.Config
	store       *store.Store
	providers   map[string]provider.Provider
	syncers     map[string]*syncer.Service
	activeProviderID string
	agg         *aggregate.Service
	mu          sync.RWMutex
	settingsMu         sync.Mutex
	settingsSrv        *http.Server
	settingsListenPort int
	view        render.ViewState
	screenSize  render.Size
	frameSnaps  map[string]frameSnapshot
	lastPNG           []byte
	frameBase         *image.RGBA
	frameBaseKey      string
	lastErrs    map[string]error
	syncing     map[string]bool
	progress    map[string]string
	exitCh        chan struct{}
	displayCh     chan displayNotify
	renderMu      sync.Mutex
	lastTouchTap  time.Time
	portraitRota  atomic.Int32 // 输入：随加速度计变化（触摸/按键）
	baselineRota  atomic.Int32 // 启动时物理握持方向（0/2）
	fbRota        atomic.Int32 // fbink currentRota；0 时常为陈旧 WM 状态
	touchMapping  atomic.Value
	touchQuirkVer atomic.Uint64
	kindleReady   atomic.Bool
	batchSyncing  atomic.Bool
	syncTimeMu    sync.RWMutex
	lastGlobalSyncAt  time.Time
	lastBatchStartedAt time.Time
	kindleUI            kindleUIState
	suppressInputUntil  atomic.Int64
	rtcMaintaining      atomic.Bool
	rtcArmed            atomic.Bool
	pendingRTCMu        sync.Mutex
	pendingRTC          *pendingRTCWake
}

type pendingRTCWake struct {
	sleptSec int
}

type displayNotify struct {
	urgent    bool
	forceFull bool
	done      chan struct{}
}

func New(roots paths.Roots) (*App, error) {
	log.Info("app init begin")
	if err := os.MkdirAll(roots.Etc, 0o755); err != nil {
		log.Error("mkdir etc: %v", err)
		return nil, err
	}
	if err := os.MkdirAll(roots.Cache, 0o755); err != nil {
		log.Error("mkdir cache: %v", err)
		return nil, err
	}

	cfg, err := config.Load(roots.Config)
	if err != nil {
		log.Error("load config: %v", err)
		return nil, err
	}
	if err := importTokenIfPresent(roots, &cfg); err != nil {
		log.Error("import token: %v", err)
		return nil, err
	}
	LogConfigLoaded(cfg)

	st, err := store.Open(roots.DB)
	if err != nil {
		log.Error("open store %s: %v", roots.DB, err)
		return nil, err
	}
	loc, _ := cfg.Location()
	providers, err := buildProviders(cfg)
	if err != nil {
		st.Close()
		log.Error("build providers: %v", err)
		return nil, err
	}
	syncers := make(map[string]*syncer.Service, len(providers))
	for id, prov := range providers {
		syncers[id] = syncer.New(prov, st)
	}

	a := &App{
		roots:      roots,
		cfg:        cfg,
		store:      st,
		providers:  providers,
		syncers:    syncers,
		agg:        aggregate.New(st, loc),
		exitCh:     make(chan struct{}, 1),
		displayCh:  make(chan displayNotify, 64),
		frameSnaps: make(map[string]frameSnapshot),
		lastErrs:   make(map[string]error),
		syncing:    make(map[string]bool),
		progress:   make(map[string]string),
		view: render.ViewState{
			Screen:       render.ScreenSummary,
			ChartMetric:  render.MetricToken,
			Orientation:  detectOrientation(),
			ProviderID:   provider.CursorID,
			SupportsCost: true,
		},
	}
	for id, svc := range syncers {
		a.attachSyncProgress(id, svc)
	}
	a.loadActiveProvider(context.Background())
	a.loadGlobalSyncTimes(context.Background())
	log.Info("app init ok orientation=%s screen=summary provider=%s", a.view.Orientation, a.activeProviderID)
	return a, nil
}

func (a *App) Close() error {
	a.stopSettingsServer()
	return a.store.Close()
}

func (a *App) Config() config.Config { return a.cfg }

func (a *App) Roots() paths.Roots { return a.roots }

func (a *App) frameSize() render.Size {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.screenSize.Width > 0 && a.screenSize.Height > 0 {
		return a.screenSize
	}
	return render.DefaultSize(a.view.Orientation)
}

func (a *App) SetScreenSize(w, h int) {
	a.mu.Lock()
	if a.screenSize.Width != w || a.screenSize.Height != h {
		a.invalidateFrameBaseLocked()
	}
	a.screenSize = render.Size{Width: w, Height: h}
	a.mu.Unlock()
}

func (a *App) invalidateFrameBaseLocked() {
	a.frameBase = nil
	a.frameBaseKey = ""
}

func (a *App) View() render.ViewState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.view
}

func (a *App) SetView(fn func(*render.ViewState)) {
	a.mu.Lock()
	fn(&a.view)
	a.mu.Unlock()
	a.RefreshFrame()
}

func (a *App) SetViewUrgent(fn func(*render.ViewState)) {
	a.mu.Lock()
	fn(&a.view)
	a.invalidateFrameBaseLocked()
	a.mu.Unlock()
	go a.refreshFrameViewOnly(true)
}

func (a *App) RefreshFrame() {
	a.refreshFrame(false)
}

func (a *App) suppressInputFor(d time.Duration) {
	a.suppressInputUntil.Store(time.Now().Add(d).UnixNano())
}

func (a *App) clearInputSuppress() {
	a.suppressInputUntil.Store(0)
}

func (a *App) armPendingRTCWake(sleptSec int) {
	a.pendingRTCMu.Lock()
	a.pendingRTC = &pendingRTCWake{sleptSec: sleptSec}
	a.pendingRTCMu.Unlock()
}

func (a *App) cancelPendingRTCWake() bool {
	a.pendingRTCMu.Lock()
	defer a.pendingRTCMu.Unlock()
	if a.pendingRTC == nil {
		return false
	}
	a.pendingRTC = nil
	return true
}

func (a *App) takePendingRTCWake() (int, bool) {
	a.pendingRTCMu.Lock()
	defer a.pendingRTCMu.Unlock()
	if a.pendingRTC == nil {
		return 0, false
	}
	slept := a.pendingRTC.sleptSec
	a.pendingRTC = nil
	return slept, true
}

func (a *App) inputSuppressed() bool {
	until := a.suppressInputUntil.Load()
	return until > 0 && time.Now().UnixNano() < until
}

func (a *App) refreshFrameViewOnly(urgent bool) {
	a.refreshFrameOpts(urgent, true, false)
}

func (a *App) refreshFrame(urgent bool) {
	a.refreshFrameOpts(urgent, false, false)
}

func (a *App) refreshFrameOpts(urgent, viewOnly, forceFull bool) {
	if urgent {
		a.renderMu.Lock()
	} else if !a.renderMu.TryLock() {
		log.Info("render frame skipped busy urgent=false")
		return
	}
	defer a.renderMu.Unlock()

	start := time.Now()
	a.mu.RLock()
	screen := a.view.Screen
	a.mu.RUnlock()
	if screen == render.ScreenSummary {
		a.renderSummaryFrame(urgent, viewOnly, forceFull, start)
		return
	}
	a.renderProviderFrame(urgent, viewOnly, forceFull, start)
}

func (a *App) renderProviderFrame(urgent, viewOnly, forceFull bool, start time.Time) {
	ctx := context.Background()

	var (
		dash  aggregate.Dashboard
		line  []aggregate.LinePoint
		heat  aggregate.HeatmapStats
		aggMs int64
	)

	a.mu.RLock()
	providerID := a.activeProviderIDLocked()
	if a.view.Screen == render.ScreenProvider && a.view.ProviderID != "" {
		providerID = a.view.ProviderID
	}
	snap := a.frameSnaps[providerID]
	view := a.view
	prov := a.providers[providerID]
	cachedBase := a.frameBase
	cachedKey := a.frameBaseKey
	key := frameBaseKey(render.ScreenProvider, providerID)
	a.mu.RUnlock()

	if viewOnly && snap.fullValid {
		dash, line, heat = snap.dash, snap.line, snap.heat
	} else {
		aggStart := time.Now()
		dash, _ = a.agg.Build(ctx, providerID)
		line, _ = a.agg.LineSeries(ctx, providerID, 30)
		size := a.frameSize()
		heatWeeks := render.HeatmapWeeksForWidth(size.Width - render.PadX*2)
		heat, _ = a.agg.Heatmap(ctx, providerID, heatWeeks)
		aggMs = time.Since(aggStart).Milliseconds()

		a.mu.Lock()
		a.frameSnaps[providerID] = frameSnapshot{
			dash: dash, line: line, heat: heat,
			dashValid: true, fullValid: true,
		}
		a.mu.Unlock()
	}

	a.mu.RLock()
	if msg, ok := a.progress[providerID]; ok && msg != "" {
		dash.SyncMessage = msg
	}
	dash.SyncStatus = a.providerSyncStatusForDashLocked(providerID, dash)
	a.mu.RUnlock()

	size := a.frameSize()
	if prov != nil {
		view.ProviderName = prov.DisplayName()
		view.ProviderID = providerID
		view.SupportsCost = prov.Capabilities().SupportsCost
	}

	var base *image.RGBA
	pngStart := time.Now()
	if viewOnly && snap.fullValid && cachedBase != nil && cachedKey == key {
		base = cachedBase
	} else {
		base = render.DrawFrame(dash, line, heat, view, size)
		a.mu.Lock()
		if a.view.Screen == render.ScreenProvider && a.view.ProviderID == providerID {
			a.frameBase = base
			a.frameBaseKey = key
		}
		a.mu.Unlock()
	}

	a.encodeAndDisplayFrame(base, providerID, urgent, viewOnly, forceFull, start, aggMs, pngStart)
}

func (a *App) renderSummaryFrame(urgent, viewOnly, forceFull bool, start time.Time) {
	ctx := context.Background()
	var aggMs int64

	a.mu.RLock()
	view := a.view
	cachedBase := a.frameBase
	cachedKey := a.frameBaseKey
	snapsReady := a.allSummarySnapsReadyLocked()
	key := frameBaseKey(render.ScreenSummary, "")
	a.mu.RUnlock()

	var overview aggregate.Overview
	if viewOnly && snapsReady {
		overview = a.buildOverview(ctx, true)
	} else {
		aggStart := time.Now()
		overview = a.buildOverview(ctx, false)
		aggMs = time.Since(aggStart).Milliseconds()
	}

	size := a.frameSize()
	var base *image.RGBA
	pngStart := time.Now()
	if viewOnly && snapsReady && cachedBase != nil && cachedKey == key {
		base = cachedBase
	} else {
		base = render.DrawSummaryFrame(overview, view, size)
		a.mu.Lock()
		if a.view.Screen == render.ScreenSummary {
			a.frameBase = base
			a.frameBaseKey = key
		}
		a.mu.Unlock()
	}

	a.encodeAndDisplayFrame(base, "summary", urgent, viewOnly, forceFull, start, aggMs, pngStart)
}

func (a *App) encodeAndDisplayFrame(base *image.RGBA, logID string, urgent, viewOnly, forceFull bool, start time.Time, aggMs int64, pngStart time.Time) {
	inputRota := a.currentPortraitRota()
	baseline := int(a.baselineRota.Load())
	flipRota := render.PortraitRotaForDisplay(inputRota, 0, baseline)
	img := render.PortraitOrient(base, flipRota)
	png, err := render.EncodePNG(img)
	pngMs := time.Since(pngStart).Milliseconds()
	a.mu.Lock()
	a.lastPNG = png
	if err != nil && logID != "summary" {
		a.lastErrs[logID] = err
	}
	a.mu.Unlock()
	log.Info("render frame ok id=%s urgent=%v viewOnly=%v display_rota=%d input_rota=%d fb_rota=%d baseline=%d agg_ms=%d png_ms=%d total_ms=%d err=%v",
		logID, urgent, viewOnly, flipRota, inputRota, int(a.fbRota.Load()), baseline, aggMs, pngMs, time.Since(start).Milliseconds(), err)

	a.mu.RLock()
	screen := a.view.Screen
	providerID := a.view.ProviderID
	a.mu.RUnlock()
	if logID == "summary" {
		if screen != render.ScreenSummary {
			return
		}
	} else if screen != render.ScreenProvider || providerID != logID {
		return
	}
	a.notifyDisplay(urgent, forceFull)
}

func (a *App) PNG() []byte {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastPNG
}

func (a *App) LastError() error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastErrs[a.activeProviderIDLocked()]
}

// displayBlockingError 仅在详情页且当前 provider 同步失败时阻止刷屏；概览页仍应展示（含错误状态）。
func (a *App) displayBlockingError() error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.view.Screen != render.ScreenProvider {
		return nil
	}
	id := a.activeProviderIDLocked()
	if id == "" {
		return nil
	}
	return a.lastErrs[id]
}

func (a *App) sendDisplayNotify(n displayNotify) bool {
	for i := 0; i < 8; i++ {
		select {
		case a.displayCh <- n:
			return true
		default:
		}
		select {
		case old := <-a.displayCh:
			if n.urgent {
				old.urgent = true
			}
			if n.forceFull {
				old.forceFull = true
			}
			if n.done != nil {
				old.done = n.done
			}
			n = old
		default:
			log.Warn("notify display queue full urgent=%v forceFull=%v", n.urgent, n.forceFull)
			return false
		}
	}
	return false
}

func (a *App) notifyDisplay(urgent, forceFull bool) {
	a.sendDisplayNotify(displayNotify{urgent: urgent, forceFull: forceFull})
}

func (a *App) notifyDisplayWait(urgent, forceFull bool, timeout time.Duration) {
	done := make(chan struct{}, 1)
	if !a.sendDisplayNotify(displayNotify{urgent: urgent, forceFull: forceFull, done: done}) {
		return
	}
	select {
	case <-done:
	case <-time.After(timeout):
		log.Warn("display flush timeout after %s", timeout)
	}
}

func (a *App) DoSync(ctx context.Context) error {
	return a.syncAllProvidersBatch(ctx, syncCauseManual)
}

func (a *App) syncerFor(id string) *syncer.Service {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.syncers[id]
}

func (a *App) startSyncProviderAsync(ctx context.Context, id string) bool {
	if !a.tryBeginSync(id) {
		return false
	}
	go func() {
		defer a.finishSync(id)
		svc := a.syncerFor(id)
		if svc == nil {
			return
		}
		err := svc.Run(ctx, "auto")
		a.mu.Lock()
		a.lastErrs[id] = err
		if id == a.activeProviderIDLocked() && err == nil {
			a.view.SyncStatus = "就绪"
		}
		a.mu.Unlock()
	}()
	return true
}

func (a *App) startSyncProvidersBatchAsync() bool {
	if a.batchSyncing.Load() || a.IsSyncing() {
		return false
	}
	go func() {
		_ = a.syncAllProvidersBatch(context.Background(), syncCauseManual)
	}()
	return true
}

func (a *App) tryBeginSync(id string) bool {
	a.mu.Lock()
	if a.batchSyncing.Load() || a.syncing[id] {
		a.mu.Unlock()
		return false
	}
	a.syncing[id] = true
	if id == a.activeProviderIDLocked() && a.view.Screen == render.ScreenProvider {
		a.view.SyncStatus = "同步中"
	}
	a.mu.Unlock()
	if id == a.activeProviderIDLocked() && a.view.Screen == render.ScreenProvider && !render.KindleUI() {
		a.RefreshFrame()
	}
	return true
}

func (a *App) finishSync(id string) {
	a.finishSyncProvider(id)
	a.mu.Lock()
	a.invalidateFrameBaseLocked()
	a.mu.Unlock()
	a.refreshAfterSingleSync(id)
}

func (a *App) IsSyncing() bool {
	if a.batchSyncing.Load() {
		return true
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, v := range a.syncing {
		if v {
			return true
		}
	}
	return false
}

func (a *App) RunDev(ctx context.Context, addr string) error {
	a.RefreshFrame()
	go a.devBackgroundSync(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("/", servePreview)
	mux.HandleFunc("/frame", func(w http.ResponseWriter, r *http.Request) {
		png := a.PNG()
		if len(png) == 0 {
			a.RefreshFrame()
			png = a.PNG()
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(png)
	})
	mux.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		a.mu.RLock()
		id := a.activeProviderIDLocked()
		a.mu.RUnlock()
		dash, _ := a.agg.Build(ctx, id)
		_ = json.NewEncoder(w).Encode(dash)
	})
	mux.HandleFunc("/api/layout", func(w http.ResponseWriter, r *http.Request) {
		a.mu.RLock()
		orient := a.view.Orientation
		screen := a.view.Screen
		active := a.view.SettingsActive
		url := a.view.SettingsURL
		metric := a.view.ChartMetric
		providerID := a.activeProviderIDLocked()
		if screen == render.ScreenProvider && a.view.ProviderID != "" {
			providerID = a.view.ProviderID
		}
		supportsCost := a.view.SupportsCost
		name := a.view.ProviderName
		a.mu.RUnlock()
		size := a.frameSize()
		regions := render.TopControlsHitRegions(screen, name)
		resp := map[string]any{
			"width":           size.Width,
			"height":          size.Height,
			"orientation":     orient,
			"screen":          string(screen),
			"syncing":         a.IsSyncing(),
			"settings_active": active,
			"settings_url":    url,
			"provider_id":     providerID,
			"regions":         regions,
		}
		if screen == render.ScreenProvider {
			resp["chart_metric"] = metric
			resp["supports_cost"] = supportsCost
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/api/action", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		switch name {
		case "refresh":
			started := a.startSyncProvidersBatchAsync()
			writeRefreshAction(w, started, started || a.IsSyncing())
		case "toggle_metric":
			a.mu.RLock()
			screen := a.view.Screen
			supportsCost := a.view.SupportsCost
			a.mu.RUnlock()
			if screen != render.ScreenProvider || !supportsCost {
				writeActionOK(w)
				break
			}
			a.SetView(func(v *render.ViewState) {
				if v.ChartMetric == render.MetricToken {
					v.ChartMetric = render.MetricCost
				} else {
					v.ChartMetric = render.MetricToken
				}
			})
			writeActionOK(w)
		case "toggle_provider", "cycle_screen":
			a.cycleScreen()
			writeActionOK(w)
		case "set_provider":
			id := r.URL.Query().Get("id")
			a.setActiveProvider(id)
			writeActionOK(w)
		case "toggle_settings":
			err := a.ToggleSettingsServer()
			active, url := a.viewSettingsState()
			writeSettingsAction(w, active, url, err)
		case "orientation":
			a.SetView(func(v *render.ViewState) {
				if v.Orientation == "landscape" {
					v.Orientation = "portrait"
				} else {
					v.Orientation = "landscape"
				}
			})
			writeActionOK(w)
		case "exit":
			a.requestExit()
			writeActionOK(w)
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":      a.LastError() == nil,
			"error":   fmt.Sprintf("%v", a.LastError()),
			"syncing": a.IsSyncing(),
		})
	})

	srv := &http.Server{Addr: addr, Handler: mux}
	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()
	fmt.Printf("kiage dev preview http://%s (Oasis %s)\n", addr, a.view.Orientation)
	return srv.ListenAndServe()
}

func (a *App) RunLoop(ctx context.Context) error {
	a.RefreshFrame()
	go a.devBackgroundSync(ctx)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	return nil
}

func importTokenIfPresent(roots paths.Roots, cfg *config.Config) error {
	data, err := os.ReadFile(roots.Import)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	token := string(bytesTrim(data))
	if token == "" {
		return nil
	}
	cfg.Cursor.SessionToken = token
	if err := config.Save(roots.Config, *cfg); err != nil {
		return err
	}
	return os.Remove(roots.Import)
}

func bytesTrim(b []byte) string {
	for len(b) > 0 && (b[0] == ' ' || b[0] == '\n' || b[0] == '\r' || b[0] == '\t') {
		b = b[1:]
	}
	for len(b) > 0 && (b[len(b)-1] == ' ' || b[len(b)-1] == '\n' || b[len(b)-1] == '\r' || b[len(b)-1] == '\t') {
		b = b[:len(b)-1]
	}
	return string(b)
}

func detectOrientation() string {
	if os.Getenv("KIAGE_PORTRAIT") == "1" {
		return "portrait"
	}
	if os.Getenv("KIAGE_LANDSCAPE") == "1" {
		return "landscape"
	}
	if runtime.GOOS == "linux" {
		return "portrait"
	}
	return "landscape"
}

func (a *App) viewSettingsState() (active bool, url string) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.view.SettingsActive, a.view.SettingsURL
}

func writeActionOK(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func writeSettingsAction(w http.ResponseWriter, active bool, url string, err error) {
	w.Header().Set("Content-Type", "application/json")
	resp := map[string]any{
		"ok":              err == nil,
		"settings_active": active,
		"settings_url":    url,
	}
	if err != nil {
		resp["error"] = err.Error()
		w.WriteHeader(http.StatusConflict)
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func writeRefreshAction(w http.ResponseWriter, started, syncing bool) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":      started,
		"started": started,
		"skipped": !started && syncing,
		"syncing": syncing,
	})
}
