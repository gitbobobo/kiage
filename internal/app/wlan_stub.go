//go:build !linux

package app

import (
	"context"
	"time"
)

func wlanConnected() bool { return true }

func wlanEnsureOn()                 {}
func wlanEnsureOnAfterResume()      {}
func wlanEnsureOff()                {}
func wlanConnectAfterResume(context.Context) bool { return true }

func waitForWLAN(ctx context.Context, timeout time.Duration) bool { return true }
