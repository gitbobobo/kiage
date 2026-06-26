//go:build !linux

package app

import "context"

type kindleUIState struct {
	awesomeStopped bool
	pillowDisabled bool
}

func (a *App) initKindleUIState() {}

func (a *App) runPowerManager(ctx context.Context) {
	<-ctx.Done()
}

func maybeKeepScreenAwake() {}
func releaseScreenAwake()   {}
func rtcKeepAwake(bool)     {}
