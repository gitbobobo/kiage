package app

import (
	"os"

	"github.com/godbobo/kiage/internal/config"
	"github.com/godbobo/kiage/internal/log"
	"github.com/godbobo/kiage/internal/paths"
	"github.com/godbobo/kiage/internal/render"
)

func LogRunEnvironment(roots paths.Roots) {
	cwd, _ := os.Getwd()
	log.Info("kiage run root=%s cwd=%s", roots.Root, cwd)
	log.Info("paths config=%s db=%s log=%s", roots.Config, roots.DB, roots.Log)
	log.Info("env KIAGE_ROOT=%q KIAGE_FBINK=%q KIAGE_TOUCH_DEV=%q KIAGE_PORTRAIT=%q",
		os.Getenv("KIAGE_ROOT"),
		os.Getenv("KIAGE_FBINK"),
		os.Getenv("KIAGE_TOUCH_DEV"),
		os.Getenv("KIAGE_PORTRAIT"),
	)
}

func LogConfigLoaded(cfg config.Config) {
	if render.KindleUI() {
		log.Info("config timezone=%s sync=wake+rtc(1h) stale=%s token=%s",
			cfg.Timezone,
			globalSyncStaleAfter,
			config.RedactToken(cfg.Cursor.SessionToken),
		)
		return
	}
	log.Info("config timezone=%s refresh_interval_sec=%d token=%s",
		cfg.Timezone,
		cfg.RefreshIntervalSec,
		config.RedactToken(cfg.Cursor.SessionToken),
	)
}
