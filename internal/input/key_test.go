package input

import (
	"testing"
	"time"
)

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

func TestScreenDownKeyDPad(t *testing.T) {
	if ScreenDownKey(KeyUp, 0) || !ScreenDownKey(KeyDown, 0) {
		t.Fatal("rota=0 dpad")
	}
	if ScreenDownKey(KeyDown, 2) || !ScreenDownKey(KeyUp, 2) {
		t.Fatal("rota=2 dpad")
	}
	if !ScreenDownKey(KeyDown, 1) {
		t.Fatal("rota normalize")
	}
	if ScreenDownKey(126, 0) {
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

func TestScreenDownKeyPageButtons(t *testing.T) {
	t.Setenv("KIAGE_SCREEN_UP_KEY", "pageup")
	if ScreenDownKey(KeyPageUp, 0) || !ScreenDownKey(KeyPageDown, 0) {
		t.Fatal("rota=0 page")
	}
	if ScreenDownKey(KeyPageDown, 2) || !ScreenDownKey(KeyPageUp, 2) {
		t.Fatal("rota=2 page")
	}

	t.Setenv("KIAGE_SCREEN_UP_KEY", "pagedown")
	if ScreenDownKey(KeyPageDown, 0) || !ScreenDownKey(KeyPageUp, 0) {
		t.Fatal("inverted rota=0")
	}
	if ScreenDownKey(KeyPageUp, 2) || !ScreenDownKey(KeyPageDown, 2) {
		t.Fatal("inverted rota=2")
	}
}

func TestClickDetectorSingle(t *testing.T) {
	d := newClickDetector()
	d.window = 50 * time.Millisecond
	defer d.Stop()

	ch := make(chan ScreenKeyAction, 1)
	emit := func(a ScreenKeyAction) { ch <- a }

	d.onRelease(clickUp, emit)
	select {
	case got := <-ch:
		if got != ScreenUpSingle {
			t.Fatalf("got %v want up-single", got)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout waiting for up-single")
	}
}

func TestClickDetectorDouble(t *testing.T) {
	d := newClickDetector()
	d.window = 50 * time.Millisecond
	defer d.Stop()

	ch := make(chan ScreenKeyAction, 2)
	emit := func(a ScreenKeyAction) { ch <- a }

	d.onRelease(clickDown, emit)
	time.Sleep(20 * time.Millisecond)
	d.onRelease(clickDown, emit)

	select {
	case got := <-ch:
		if got != ScreenDownDouble {
			t.Fatalf("got %v want down-double", got)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout waiting for down-double")
	}

	select {
	case got := <-ch:
		t.Fatalf("unexpected extra action: %v", got)
	case <-time.After(80 * time.Millisecond):
	}
}

func TestClickDetectorDirectionsIndependent(t *testing.T) {
	d := newClickDetector()
	d.window = 50 * time.Millisecond
	defer d.Stop()

	ch := make(chan ScreenKeyAction, 2)
	emit := func(a ScreenKeyAction) { ch <- a }

	d.onRelease(clickUp, emit)
	d.onRelease(clickDown, emit)

	got := map[ScreenKeyAction]bool{}
	for i := 0; i < 2; i++ {
		select {
		case a := <-ch:
			got[a] = true
		case <-time.After(200 * time.Millisecond):
			t.Fatalf("timeout, got %v", got)
		}
	}
	if !got[ScreenUpSingle] || !got[ScreenDownSingle] {
		t.Fatalf("got %v want up-single and down-single", got)
	}
}
