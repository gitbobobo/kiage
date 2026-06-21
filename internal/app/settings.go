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
	"github.com/godbobo/kiage/internal/log"
	"github.com/godbobo/kiage/internal/render"
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

func (a *App) ToggleSettingsServer() error {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()

	if a.settingsSrv != nil {
		err := a.stopSettingsServerLocked()
		a.refreshAfterSettingsChange()
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

	url := localLANURL(listenPort)
	if render.KindleUI() && url == fmt.Sprintf("http://127.0.0.1:%d", listenPort) {
		log.Warn("settings server: no wlan IP, check WiFi is connected")
	}

	kindleFirewallOpen(listenPort)

	a.settingsSrv = srv
	a.settingsListenPort = listenPort

	a.mu.Lock()
	a.view.SettingsActive = true
	a.view.SettingsURL = url
	a.mu.Unlock()

	log.Info("settings server started port=%d url=%s", listenPort, url)

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Error("settings server stopped unexpectedly: %v", err)
			a.settingsMu.Lock()
			if a.settingsSrv == srv {
				a.stopSettingsServerLocked()
			}
			a.settingsMu.Unlock()
			a.refreshAfterSettingsChange()
		}
	}()

	a.refreshAfterSettingsChange()
	return nil
}

func (a *App) refreshAfterSettingsChange() {
	if render.KindleUI() {
		go a.refreshFrame(true)
		return
	}
	a.RefreshFrame()
}

func (a *App) stopSettingsServer() {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	_ = a.stopSettingsServerLocked()
	a.refreshAfterSettingsChange()
}

func (a *App) stopSettingsServerLocked() error {
	if a.settingsSrv == nil {
		return nil
	}
	port := a.settingsListenPort

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := a.settingsSrv.Shutdown(ctx)
	a.settingsSrv = nil
	a.settingsListenPort = 0

	kindleFirewallClose(port)

	a.mu.Lock()
	a.view.SettingsActive = false
	a.view.SettingsURL = ""
	a.mu.Unlock()

	log.Info("settings server stopped port=%d", port)
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
			"glm_key_hint":         config.RedactToken(cfg.GLM.APIKey),
			"minimax_key_hint":     config.RedactToken(cfg.MiniMax.APIKey),
			"kimi_key_hint":        config.RedactToken(cfg.Kimi.APIKey),
			"timezone":             cfg.Timezone,
			"refresh_interval_sec": cfg.RefreshIntervalSec,
		})
	case http.MethodPost:
		var body struct {
			SessionToken       string `json:"session_token"`
			GLMAPIKey          string `json:"glm_api_key"`
			MiniMaxAPIKey      string `json:"minimax_api_key"`
			KimiAPIKey         string `json:"kimi_api_key"`
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
		if body.GLMAPIKey != "" {
			cfg.GLM.APIKey = body.GLMAPIKey
		}
		if body.MiniMaxAPIKey != "" {
			cfg.MiniMax.APIKey = body.MiniMaxAPIKey
		}
		if body.KimiAPIKey != "" {
			cfg.Kimi.APIKey = body.KimiAPIKey
		}
		if body.Timezone != "" {
			cfg.Timezone = body.Timezone
		}
		if body.RefreshIntervalSec >= 60 {
			if body.RefreshIntervalSec > config.MaxRefreshIntervalSec {
				body.RefreshIntervalSec = config.MaxRefreshIntervalSec
			}
			cfg.RefreshIntervalSec = body.RefreshIntervalSec
		}
		a.mu.Unlock()

		providers, err := buildProviders(cfg)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		if err := cfg.Validate(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		if err := config.Save(a.roots.Config, cfg); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		if err := a.reloadProviders(cfg, providers); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":                   true,
			"token_hint":           config.RedactToken(cfg.Cursor.SessionToken),
			"glm_key_hint":         config.RedactToken(cfg.GLM.APIKey),
			"minimax_key_hint":     config.RedactToken(cfg.MiniMax.APIKey),
			"kimi_key_hint":        config.RedactToken(cfg.Kimi.APIKey),
			"timezone":             cfg.Timezone,
			"refresh_interval_sec": cfg.RefreshIntervalSec,
		})
		a.RefreshFrame()
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
