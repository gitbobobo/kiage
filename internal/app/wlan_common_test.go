package app

import "testing"

func TestWlanSanitizeSSID(t *testing.T) {
	if got := wlanSanitizeSSID("[routebobo]"); got != "routebobo" {
		t.Fatalf("got %q", got)
	}
	if got := wlanSanitizeSSID("routebobo"); got != "routebobo" {
		t.Fatalf("got %q", got)
	}
}
