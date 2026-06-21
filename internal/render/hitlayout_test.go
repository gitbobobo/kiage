package render

import "testing"

func TestTopControlsHitRegionsProviderTitle(t *testing.T) {
	t.Setenv("KIAGE_PORTRAIT", "1")
	regions := TopControlsHitRegions(ScreenProvider, "Cursor")

	cx := regions.ProviderTitle.X + regions.ProviderTitle.W/2
	cy := regions.ProviderTitle.Y + regions.ProviderTitle.H/2
	if !regions.ProviderTitle.Contains(cx, cy) {
		t.Fatal("provider title should contain center")
	}
	if regions.ProviderTitle.Contains(900, 10) {
		t.Fatal("top-right should not hit provider title")
	}
}

func TestTopControlsHitRegionsSummaryEmpty(t *testing.T) {
	regions := TopControlsHitRegions(ScreenSummary, "用量概览")
	if regions.ProviderTitle.W != 0 || regions.ProviderTitle.H != 0 {
		t.Fatalf("summary should have empty provider title region %+v", regions.ProviderTitle)
	}
}
