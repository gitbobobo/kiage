package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const CurrentVersion = 1

const (
	MaxCredentialLen        = 8192
	MaxRefreshIntervalSec   = 86400
)

type Config struct {
	Version            int    `json:"version"`
	Timezone           string `json:"timezone"`
	RefreshIntervalSec int    `json:"refresh_interval_sec"`
	Cursor             Cursor `json:"cursor"`
	GLM                GLM    `json:"glm"`
}

type Cursor struct {
	SessionToken string `json:"session_token"`
}

type GLM struct {
	APIKey string `json:"api_key"`
}

func Default() Config {
	return Config{
		Version:            CurrentVersion,
		Timezone:           "Asia/Shanghai",
		RefreshIntervalSec: 600,
		Cursor: Cursor{
			SessionToken: "",
		},
		GLM: GLM{
			APIKey: "",
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return migrate(cfg), nil
}

func migrate(cfg Config) Config {
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if cfg.Timezone == "" {
		cfg.Timezone = "Asia/Shanghai"
	}
	if cfg.RefreshIntervalSec <= 0 {
		cfg.RefreshIntervalSec = 600
	}
	return cfg
}

func (c Config) Location() (*time.Location, error) {
	loc, err := time.LoadLocation(c.Timezone)
	if err != nil {
		return nil, fmt.Errorf("invalid timezone %q: %w", c.Timezone, err)
	}
	return loc, nil
}

func Save(path string, cfg Config) error {
	if cfg.Version == 0 {
		cfg.Version = CurrentVersion
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (c Config) Validate() error {
	if c.RefreshIntervalSec > 0 && c.RefreshIntervalSec < 60 {
		return errors.New("refresh_interval_sec must be >= 60")
	}
	if c.RefreshIntervalSec > MaxRefreshIntervalSec {
		return fmt.Errorf("refresh_interval_sec must be <= %d", MaxRefreshIntervalSec)
	}
	if len(c.Cursor.SessionToken) > MaxCredentialLen || len(c.GLM.APIKey) > MaxCredentialLen {
		return errors.New("credential too long")
	}
	if _, err := c.Location(); err != nil {
		return err
	}
	return nil
}

func RedactToken(token string) string {
	if len(token) <= 8 {
		return "****"
	}
	return "..." + token[len(token)-8:]
}
