package kimi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/godbobo/kiage/internal/provider"
)

func TestFetchSummaryFromFixture(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != usagesPath {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		data, _ := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "fixtures", "kimi-usages.json"))
		w.Write(data)
	}))
	defer srv.Close()

	loc, _ := time.LoadLocation("Asia/Shanghai")
	c := NewWithHTTP("test-key", loc, srv.Client(), srv.URL)
	sum, err := c.FetchSummary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if sum.PlanName != "Moderato" {
		t.Fatalf("plan name: %q", sum.PlanName)
	}
	if len(sum.Bars) != 2 {
		t.Fatalf("want 2 bars got %d", len(sum.Bars))
	}
	if sum.Bars[0].Label != provider.LabelIntervalQuota || sum.Bars[0].Percent != 69.5 {
		t.Fatalf("unexpected interval bar %+v", sum.Bars[0])
	}
	if sum.Bars[1].Label != provider.LabelWeeklyQuota || sum.Bars[1].Percent != 62.2 {
		t.Fatalf("unexpected weekly bar %+v", sum.Bars[1])
	}
	if sum.Bars[0].ResetAt.IsZero() || sum.Bars[1].ResetAt.IsZero() {
		t.Fatal("expected reset times on bars")
	}
}

func TestFetchSummaryUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := NewWithHTTP("bad-key", time.UTC, srv.Client(), srv.URL)
	_, err := c.FetchSummary(context.Background())
	if err != provider.ErrInvalidCredential {
		t.Fatalf("want ErrInvalidCredential got %v", err)
	}
}
