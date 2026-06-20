package display

import "testing"

func TestPickRefreshMode(t *testing.T) {
	mode, n := PickRefreshMode(false, 0, 6)
	if mode != RefreshFirst || n != 0 {
		t.Fatalf("first got mode=%d n=%d", mode, n)
	}
	mode, n = PickRefreshMode(true, 0, 6)
	if mode != RefreshPartial || n != 1 {
		t.Fatalf("partial1 got mode=%d n=%d", mode, n)
	}
	mode, n = PickRefreshMode(true, 5, 6)
	if mode != RefreshFull || n != 0 {
		t.Fatalf("full promote got mode=%d n=%d", mode, n)
	}
}
