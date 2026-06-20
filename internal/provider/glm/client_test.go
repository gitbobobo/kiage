package glm

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
		if r.URL.Path != quotaPath {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		data, _ := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "fixtures", "glm-quota.json"))
		w.Write(data)
	}))
	defer srv.Close()

	loc, _ := time.LoadLocation("Asia/Shanghai")
	c := NewWithHTTP("test-key", loc, srv.Client(), srv.URL)
	sum, err := c.FetchSummary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if sum.PlanName != "Pro" {
		t.Fatalf("want Pro got %s", sum.PlanName)
	}
	if len(sum.Bars) < 2 {
		t.Fatalf("want at least 2 bars got %d", len(sum.Bars))
	}
	if sum.Bars[0].Label != "5小时配额" || sum.Bars[0].Percent != 18 {
		t.Fatalf("unexpected primary bar %+v", sum.Bars[0])
	}
}

func TestFetchUsageEventsFromFixture(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != modelUsagePath {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		data, _ := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "fixtures", "glm-model-usage.json"))
		w.Write(data)
	}))
	defer srv.Close()

	loc, _ := time.LoadLocation("Asia/Shanghai")
	c := NewWithHTTP("test-key", loc, srv.Client(), srv.URL)
	start := time.Date(2026, 6, 20, 0, 0, 0, 0, loc)
	end := start.Add(24*time.Hour - time.Nanosecond)
	rng := provider.DateRange{Start: start, End: end}
	page, err := c.FetchUsageEvents(context.Background(), rng, 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != 4 {
		t.Fatalf("want 4 events got %d", len(page.Events))
	}
}

func TestNewEmptyKey(t *testing.T) {
	cfg := config.Default()
	c, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.FetchSummary(context.Background())
	if err != provider.ErrInvalidCredential {
		t.Fatalf("want invalid credential got %v", err)
	}
}
