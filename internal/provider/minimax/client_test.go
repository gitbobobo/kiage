package minimax

import (
	"context"
	"errors"
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
		if r.URL.Path != remainsPath {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		data, _ := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "fixtures", "minimax-remains.json"))
		w.Write(data)
	}))
	defer srv.Close()

	loc, _ := time.LoadLocation("Asia/Shanghai")
	c := NewWithHTTP("test-key", loc, srv.Client(), srv.URL)
	sum, err := c.FetchSummary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if sum.PlanName != planName {
		t.Fatalf("want %q got %s", planName, sum.PlanName)
	}
	if len(sum.Bars) != 1 {
		t.Fatalf("want 1 bar got %d", len(sum.Bars))
	}
	if sum.Bars[0].Label != "5小时配额" || sum.Bars[0].Percent != 13 {
		t.Fatalf("unexpected interval bar %+v", sum.Bars[0])
	}
	if sum.MembershipType != "interval" {
		t.Fatalf("membership type: %q", sum.MembershipType)
	}
	if sum.ResetAt.IsZero() {
		t.Fatal("expected reset at")
	}
}

func TestFetchSummaryNoGeneralModel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"model_remains":[{"model_name":"video"}],"base_resp":{"status_code":0,"status_msg":"success"}}`))
	}))
	defer srv.Close()

	loc := time.UTC
	c := NewWithHTTP("test-key", loc, srv.Client(), srv.URL)
	_, err := c.FetchSummary(context.Background())
	if !errors.Is(err, provider.ErrSchemaChanged) {
		t.Fatalf("want ErrSchemaChanged got %v", err)
	}
}

func TestFetchSummaryStatusCodeNonZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"model_remains":[],"base_resp":{"status_code":1004,"status_msg":"invalid api key"}}`))
	}))
	defer srv.Close()

	c := NewWithHTTP("bad-key", time.UTC, srv.Client(), srv.URL)
	_, err := c.FetchSummary(context.Background())
	if err != provider.ErrInvalidCredential {
		t.Fatalf("want ErrInvalidCredential got %v", err)
	}
}

func TestFetchSummaryMultipleGeneral(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"model_remains":[
				{"model_name":"general","current_interval_status":1,"current_interval_remaining_percent":50,"end_time":1700000000000},
				{"model_name":"general","current_interval_status":1,"current_interval_remaining_percent":10,"end_time":1800000000000}
			],
			"base_resp":{"status_code":0,"status_msg":"success"}
		}`))
	}))
	defer srv.Close()

	c := NewWithHTTP("test-key", time.UTC, srv.Client(), srv.URL)
	sum, err := c.FetchSummary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if sum.Bars[0].Percent != 50 {
		t.Fatalf("want first general interval used 50%% got %v", sum.Bars[0].Percent)
	}
}

func TestFetchUsageEventsEmpty(t *testing.T) {
	c := NewWithHTTP("test-key", time.UTC, nil, "")
	page, err := c.FetchUsageEvents(context.Background(), provider.DateRange{}, 2, 50)
	if err != nil {
		t.Fatal(err)
	}
	if page.Page != 2 || page.PageSize != 50 || page.HasMore || len(page.Events) != 0 {
		t.Fatalf("unexpected page %+v", page)
	}
}
