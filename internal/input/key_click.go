package input

import (
	"sync"
	"time"
)

const keyClickWindow = 300 * time.Millisecond

type clickDir int

const (
	clickUp clickDir = iota
	clickDown
)

type clickDetector struct {
	window       time.Duration
	mu           sync.Mutex
	lastRelease  [2]time.Time
	pendingTimer [2]*time.Timer
}

func newClickDetector() *clickDetector {
	return &clickDetector{window: keyClickWindow}
}

func (d *clickDetector) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := range d.pendingTimer {
		if t := d.pendingTimer[i]; t != nil {
			t.Stop()
			d.pendingTimer[i] = nil
		}
	}
}

func (d *clickDetector) onRelease(dir clickDir, emit func(ScreenKeyAction)) {
	now := time.Now()
	idx := int(dir)

	d.mu.Lock()
	if !d.lastRelease[idx].IsZero() && now.Sub(d.lastRelease[idx]) < d.window {
		if t := d.pendingTimer[idx]; t != nil {
			t.Stop()
			d.pendingTimer[idx] = nil
		}
		d.lastRelease[idx] = time.Time{}
		d.mu.Unlock()
		emit(doubleAction(dir))
		return
	}

	d.lastRelease[idx] = now
	if t := d.pendingTimer[idx]; t != nil {
		t.Stop()
	}

	var timer *time.Timer
	timer = time.AfterFunc(d.window, func() {
		d.mu.Lock()
		if d.pendingTimer[idx] != timer {
			d.mu.Unlock()
			return
		}
		d.pendingTimer[idx] = nil
		d.lastRelease[idx] = time.Time{}
		d.mu.Unlock()
		emit(singleAction(dir))
	})
	d.pendingTimer[idx] = timer
	d.mu.Unlock()
}

func singleAction(dir clickDir) ScreenKeyAction {
	if dir == clickUp {
		return ScreenUpSingle
	}
	return ScreenDownSingle
}

func doubleAction(dir clickDir) ScreenKeyAction {
	if dir == clickUp {
		return ScreenUpDouble
	}
	return ScreenDownDouble
}
