package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/godbobo/kiage/internal/aggregate"
	"github.com/godbobo/kiage/internal/config"
	"github.com/godbobo/kiage/internal/log"
	"github.com/godbobo/kiage/internal/paths"
	"github.com/godbobo/kiage/internal/provider"
	"github.com/godbobo/kiage/internal/provider/cursor"
	"github.com/godbobo/kiage/internal/render"
	"github.com/godbobo/kiage/internal/store"
	syncer "github.com/godbobo/kiage/internal/sync"
)

type App struct {
	roots       paths.Roots
	cfg         config.Config
	store       *store.Store
	prov        provider.Provider
	sync        *syncer.Service
	agg         *aggregate.Service
	mu          sync.RWMutex
	settingsMu  sync.Mutex
	settingsSrv *http.Server
	view        render.ViewState
	screenSize  render.Size
	lastPNG     []byte
	lastErr     error
	syncing     bool
	progress    string
	exitCh      chan struct{}
	displayCh   chan struct{}
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
	prov, err := cursor.New(cfg)
	if err != nil {
		st.Close()
		log.Error("cursor provider: %v", err)
		return nil, err
	}

	a := &App{
		roots:  roots,
		cfg:    cfg,
		store:  st,
		prov:   prov,
		sync:   syncer.New(prov, st),
		agg:    aggregate.New(st, loc),
		exitCh:    make(chan struct{}, 1),
		displayCh: make(chan struct{}, 1),
		view: render.ViewState{
			ChartMetric: "token",
			Orientation: detectOrientation(),
		},
	}
	a.sync.OnProgress(func(p syncer.Progress) {
		a.mu.Lock()
		a.progress = p.Message
		a.mu.Unlock()
	})
	log.Info("app init ok orientation=%s provider=%s", a.view.Orientation, prov.DisplayName())
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
	a.screenSize = render.Size{Width: w, Height: h}
	a.mu.Unlock()
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

func (a *App) PNG() []byte {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastPNG
}

func (a *App) LastError() error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastErr
}

func (a *App) RefreshFrame() {
	ctx := context.Background()
	dash, _ := a.agg.Build(ctx, provider.CursorID)
	a.mu.RLock()
	view := a.view
	a.mu.RUnlock()

	dash.SyncStatus = view.SyncStatus
	dash.SyncMessage = a.progress

	line, _ := a.agg.LineSeries(ctx, provider.CursorID, 30)
	size := a.frameSize()
	heatWeeks := render.HeatmapWeeksForWidth(size.Width - render.PadX*2)
	heat, _ := a.agg.Heatmap(ctx, provider.CursorID, heatWeeks)
	view.ProviderName = a.prov.DisplayName()
	png, err := render.RenderPNG(dash, line, heat, view, size)
	a.mu.Lock()
	a.lastPNG = png
	a.lastErr = err
	a.mu.Unlock()
	a.notifyDisplay()
}

func (a *App) notifyDisplay() {
	select {
	case a.displayCh <- struct{}{}:
	default:
	}
}

func (a *App) DoSync(ctx context.Context) error {
	if !a.tryBeginSync() {
		return nil
	}
	defer a.finishSync()

	err := a.sync.Run(ctx, "auto")
	a.mu.Lock()
	a.lastErr = err
	a.mu.Unlock()
	return err
}

func (a *App) startSyncAsync(ctx context.Context) bool {
	if !a.tryBeginSync() {
		return false
	}
	go func() {
		defer a.finishSync()
		err := a.sync.Run(ctx, "auto")
		a.mu.Lock()
		a.lastErr = err
		a.mu.Unlock()
	}()
	return true
}

func (a *App) tryBeginSync() bool {
	a.mu.Lock()
	if a.syncing {
		a.mu.Unlock()
		return false
	}
	a.syncing = true
	a.view.SyncStatus = "同步中"
	a.mu.Unlock()
	a.RefreshFrame()
	return true
}

func (a *App) finishSync() {
	a.mu.Lock()
	a.syncing = false
	if a.lastErr == nil {
		a.view.SyncStatus = "就绪"
	} else {
		a.view.SyncStatus = "错误"
	}
	a.mu.Unlock()
	a.RefreshFrame()
}

func (a *App) IsSyncing() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.syncing
}

func (a *App) RunDev(ctx context.Context, addr string) error {
	a.RefreshFrame()
	go a.backgroundSync(ctx)

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
		dash, _ := a.agg.Build(ctx, provider.CursorID)
		_ = json.NewEncoder(w).Encode(dash)
	})
	mux.HandleFunc("/api/layout", func(w http.ResponseWriter, r *http.Request) {
		a.mu.RLock()
		orient := a.view.Orientation
		active := a.view.SettingsActive
		url := a.view.SettingsURL
		metric := a.view.ChartMetric
		a.mu.RUnlock()
		size := a.frameSize()
		regions := render.TopControlsHitRegions(size, a.prov.DisplayName())
		_ = json.NewEncoder(w).Encode(map[string]any{
			"width":           size.Width,
			"height":          size.Height,
			"orientation":     orient,
			"syncing":         a.IsSyncing(),
			"settings_active": active,
			"settings_url":    url,
			"chart_metric":    metric,
			"regions":         regions,
		})
	})
	mux.HandleFunc("/api/action", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		switch name {
		case "refresh":
			started := a.startSyncAsync(context.Background())
			writeRefreshAction(w, started, started || a.IsSyncing())
		case "toggle_metric":
			a.SetView(func(v *render.ViewState) {
				if v.ChartMetric == "token" {
					v.ChartMetric = "cost"
				} else {
					v.ChartMetric = "token"
				}
			})
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
			"syncing": a.syncing,
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
	go a.backgroundSync(ctx)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	return nil
}

func (a *App) backgroundSync(ctx context.Context) {
	_ = a.DoSync(ctx)
	interval := time.Duration(a.cfg.RefreshIntervalSec) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = a.DoSync(ctx)
		}
	}
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
	// Kindle 设备竖屏
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
