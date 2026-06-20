package cursor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/godbobo/kiage/internal/config"
	"github.com/godbobo/kiage/internal/provider"
)

func TestFetchSummaryFromFixture(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/usage-summary" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		data, _ := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "fixtures", "usage-summary.json"))
		w.Write(data)
	}))
	defer srv.Close()

	loc, _ := time.LoadLocation("Asia/Shanghai")
	c := NewWithHTTP("test-token", loc, srv.Client(), srv.URL)
	sum, err := c.FetchSummary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if sum.MembershipType != "ultra" {
		t.Fatalf("want ultra got %s", sum.MembershipType)
	}
	if sum.TotalPercent != 70 {
		t.Fatalf("want 70 got %v", sum.TotalPercent)
	}
}

func TestFetchUsageEventsFromFixture(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("want POST")
		}
		data, _ := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "fixtures", "usage-events-page1.json"))
		w.Write(data)
	}))
	defer srv.Close()

	loc, _ := time.LoadLocation("Asia/Shanghai")
	c := NewWithHTTP("test-token", loc, srv.Client(), srv.URL)
	now := time.Date(2024, 6, 19, 12, 0, 0, 0, loc)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	rng := provider.DateRange{Start: start, End: start.Add(24*time.Hour - time.Nanosecond)}
	page, err := c.FetchUsageEvents(context.Background(), rng, 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != 3 {
		t.Fatalf("want 3 events got %d", len(page.Events))
	}
}
func TestConfigDefault(t *testing.T) {
	cfg := config.Default()
	if cfg.RefreshIntervalSec != 600 {
		t.Fatalf("unexpected refresh interval")
	}
}
