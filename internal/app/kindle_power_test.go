package app

import (
	"testing"
	"time"
)

func TestClassifyPowerdLine(t *testing.T) {
	a := &App{}
	cases := []struct {
		line string
		want string
	}{
		{"goingToScreenSaver 2", "suspend"},
		{"outOfScreenSaver 1", "resume"},
		{"readyToSuspend 5", "ready"},
		{"wakeupFromSuspend 0", "rtc"},
		{"com.lab126.powerd rtcWakeup", "rtc"},
		{"noise", ""},
	}
	for _, c := range cases {
		got := a.classifyPowerdLine(c.line)
		if got != c.want {
			t.Fatalf("line %q got %q want %q", c.line, got, c.want)
		}
	}
}

func TestParseReadyToSuspendDelay(t *testing.T) {
	cases := []struct {
		line string
		want int
	}{
		{"readyToSuspend 10", 10},
		{"readyToSuspend 1", 1},
		{"com.lab126.powerd readyToSuspend 3", 3},
		{"noise", -1},
	}
	for _, c := range cases {
		got := parseReadyToSuspendDelay(c.line)
		if got != c.want {
			t.Fatalf("line %q got %d want %d", c.line, got, c.want)
		}
	}
}

func TestParseWakeupSuspendSec(t *testing.T) {
	got, ok := parseWakeupSuspendSec(`wakeupFromSuspend 90`)
	if !ok || got != 90 {
		t.Fatalf("got %d ok=%v", got, ok)
	}
	_, ok = parseWakeupSuspendSec("noise")
	if ok {
		t.Fatal("expected false for noise")
	}
}

func TestRtcWakeLooksScheduled(t *testing.T) {
	if rtcWakeLooksScheduled(90, 3600, 300) {
		t.Fatal("90s should be ignored")
	}
	if !rtcWakeLooksScheduled(3401, 3600, 300) {
		t.Fatal("3401s should be accepted")
	}
	if !rtcWakeLooksScheduled(3300, 3600, 300) {
		t.Fatal("3300s should be accepted at slack boundary")
	}
	if !rtcWakeLooksScheduled(4200, 3600, 300) {
		t.Fatal("4200s should be accepted at overshoot boundary")
	}
	if rtcWakeLooksScheduled(7326, 3600, 300) {
		t.Fatal("7326s manual long sleep should not be RTC")
	}
	if rtcWakeLooksScheduled(1861, 3600, 300) {
		t.Fatal("1861s should be ignored")
	}
}

func TestGlobalSyncStale(t *testing.T) {
	a := &App{}
	if !a.globalSyncStale() {
		t.Fatal("zero time should be stale")
	}
	a.lastGlobalSyncAt = time.Now().UTC().Add(-6 * time.Minute)
	if !a.globalSyncStale() {
		t.Fatal("6m ago should be stale")
	}
	a.lastGlobalSyncAt = time.Now().UTC().Add(-2 * time.Minute)
	if a.globalSyncStale() {
		t.Fatal("2m ago should be fresh")
	}
}

func TestShouldStartBatch(t *testing.T) {
	a := &App{}
	if !a.shouldStartBatch() {
		t.Fatal("zero batch time should allow start")
	}
	a.lastBatchStartedAt = time.Now().UTC().Add(-10 * time.Second)
	if a.shouldStartBatch() {
		t.Fatal("10s ago should dedupe")
	}
	a.lastBatchStartedAt = time.Now().UTC().Add(-45 * time.Second)
	if !a.shouldStartBatch() {
		t.Fatal("45s ago should allow start")
	}
}
