package app

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/godbobo/kiage/internal/config"
	"github.com/godbobo/kiage/internal/provider/cursor"
	syncer "github.com/godbobo/kiage/internal/sync"
)

//go:embed setup.html
var setupHTML []byte

func settingsPort() int {
	if p := os.Getenv("KIAGE_SETUP_PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			return n
		}
	}
	return 8765
}

func localLANURL(port int) string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return fmt.Sprintf("http://127.0.0.1:%d", port)
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok || ipnet.IP.IsLoopback() {
				continue
			}
			if ip4 := ipnet.IP.To4(); ip4 != nil {
				return fmt.Sprintf("http://%s:%d", ip4.String(), port)
			}
		}
	}
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

func (a *App) ToggleSettingsServer() error {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()

	if a.settingsSrv != nil {
		err := a.stopSettingsServerLocked()
		a.RefreshFrame()
		return err
	}

	port := settingsPort()
	mux := http.NewServeMux()
	mux.HandleFunc("/", a.serveSetup)
	mux.HandleFunc("/api/config", a.handleConfigAPI)

	var ln net.Listener
	var listenPort int
	for p := port; p < port+10; p++ {
		var err error
		ln, err = net.Listen("tcp", fmt.Sprintf(":%d", p))
		if err == nil {
			listenPort = p
			break
		}
	}
	if ln == nil {
		return fmt.Errorf("启动配置服务失败: 端口 %d-%d 均被占用", port, port+9)
	}

	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	a.settingsSrv = srv
	url := localLANURL(listenPort)

	a.mu.Lock()
	a.view.SettingsActive = true
	a.view.SettingsURL = url
	a.mu.Unlock()

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			a.settingsMu.Lock()
			if a.settingsSrv == srv {
				a.stopSettingsServerLocked()
			}
			a.settingsMu.Unlock()
			a.RefreshFrame()
		}
	}()

	a.RefreshFrame()
	return nil
}

func (a *App) stopSettingsServer() {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	_ = a.stopSettingsServerLocked()
	a.RefreshFrame()
}

func (a *App) stopSettingsServerLocked() error {
	if a.settingsSrv == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := a.settingsSrv.Shutdown(ctx)
	a.settingsSrv = nil

	a.mu.Lock()
	a.view.SettingsActive = false
	a.view.SettingsURL = ""
	a.mu.Unlock()
	return err
}

func (a *App) serveSetup(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(setupHTML)
}

func (a *App) handleConfigAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.mu.RLock()
		cfg := a.cfg
		a.mu.RUnlock()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token_hint":           config.RedactToken(cfg.Cursor.SessionToken),
			"timezone":             cfg.Timezone,
			"refresh_interval_sec": cfg.RefreshIntervalSec,
		})
	case http.MethodPost:
		var body struct {
			SessionToken       string `json:"session_token"`
			Timezone           string `json:"timezone"`
			RefreshIntervalSec int    `json:"refresh_interval_sec"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
			return
		}
		a.mu.Lock()
		cfg := a.cfg
		if body.SessionToken != "" {
			cfg.Cursor.SessionToken = body.SessionToken
		}
		if body.Timezone != "" {
			cfg.Timezone = body.Timezone
		}
		if body.RefreshIntervalSec >= 60 {
			cfg.RefreshIntervalSec = body.RefreshIntervalSec
		}
		a.mu.Unlock()

		if err := config.Save(a.roots.Config, cfg); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		if err := a.reloadProvider(cfg); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":                   true,
			"token_hint":           config.RedactToken(cfg.Cursor.SessionToken),
			"timezone":             cfg.Timezone,
			"refresh_interval_sec": cfg.RefreshIntervalSec,
		})
		a.RefreshFrame()
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *App) reloadProvider(cfg config.Config) error {
	prov, err := cursor.New(cfg)
	if err != nil {
		return err
	}
	syncSvc := syncer.New(prov, a.store)
	syncSvc.OnProgress(func(p syncer.Progress) {
		a.mu.Lock()
		a.progress = p.Message
		a.mu.Unlock()
	})

	a.mu.Lock()
	a.cfg = cfg
	a.prov = prov
	a.sync = syncSvc
	a.mu.Unlock()
	return nil
}
