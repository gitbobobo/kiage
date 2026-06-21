package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/godbobo/kiage/internal/provider"
)

func TestLoadSummaryEmptyBarsForMiniMax(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ctx := context.Background()
	sum := provider.Summary{PlanName: "Token Plan"}
	if err := st.SaveSummary(ctx, provider.MiniMaxID, sum); err != nil {
		t.Fatal(err)
	}
	row, ok, err := st.LoadSummary(ctx, provider.MiniMaxID)
	if err != nil || !ok {
		t.Fatalf("load: ok=%v err=%v", ok, err)
	}
	if len(row.Bars) != 0 {
		t.Fatalf("want empty bars got %+v", row.Bars)
	}
}

func TestLoadSummaryEmptyBarsFallbackCursor(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ctx := context.Background()
	sum := provider.Summary{
		PlanName:        "Pro",
		TotalPercent:    10,
		ComposerPercent: 20,
		APIPercent:      30,
	}
	if err := st.SaveSummary(ctx, provider.CursorID, sum); err != nil {
		t.Fatal(err)
	}
	row, ok, err := st.LoadSummary(ctx, provider.CursorID)
	if err != nil || !ok {
		t.Fatalf("load: ok=%v err=%v", ok, err)
	}
	if len(row.Bars) != 3 {
		t.Fatalf("want 3 cursor bars got %d", len(row.Bars))
	}
	if row.Bars[0].Percent != 10 {
		t.Fatalf("unexpected bar %+v", row.Bars[0])
	}
}
