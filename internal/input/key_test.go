package input

import "testing"

func TestScreenUpKeyDPad(t *testing.T) {
	if !ScreenUpKey(KeyUp, 0) || ScreenUpKey(KeyDown, 0) {
		t.Fatal("rota=0 dpad")
	}
	if !ScreenUpKey(KeyDown, 2) || ScreenUpKey(KeyUp, 2) {
		t.Fatal("rota=2 dpad")
	}
	if !ScreenUpKey(KeyUp, 1) || !ScreenUpKey(KeyUp, 99) {
		t.Fatal("rota normalize")
	}
	if ScreenUpKey(126, 0) {
		t.Fatal("unknown key")
	}
}

func TestScreenUpKeyPageButtons(t *testing.T) {
	t.Setenv("KIAGE_SCREEN_UP_KEY", "pageup")
	if !ScreenUpKey(KeyPageUp, 0) || ScreenUpKey(KeyPageDown, 0) {
		t.Fatal("rota=0 page")
	}
	if !ScreenUpKey(KeyPageDown, 2) || ScreenUpKey(KeyPageUp, 2) {
		t.Fatal("rota=2 page")
	}

	t.Setenv("KIAGE_SCREEN_UP_KEY", "pagedown")
	if !ScreenUpKey(KeyPageDown, 0) || !ScreenUpKey(KeyPageUp, 2) {
		t.Fatal("inverted baseline")
	}
}
