package syncer

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/godbobo/kiage/internal/provider"
	"github.com/godbobo/kiage/internal/store"
)

type fakeProvider struct {
	id       string
	caps     provider.Capabilities
	sumCalls int
	evCalls  int
	sumErr   error
}

func (f *fakeProvider) ID() string { return f.id }

func (f *fakeProvider) DisplayName() string { return f.id }

func (f *fakeProvider) Capabilities() provider.Capabilities { return f.caps }

func (f *fakeProvider) Timezone() *time.Location { return time.UTC }

func (f *fakeProvider) ValidateCredentials(ctx context.Context) error { return nil }

func (f *fakeProvider) FetchSummary(ctx context.Context) (provider.Summary, error) {
	f.sumCalls++
	if f.sumErr != nil {
		return provider.Summary{}, f.sumErr
	}
	return provider.Summary{PlanName: "Token Plan"}, nil
}

func (f *fakeProvider) FetchUsageEvents(ctx context.Context, rng provider.DateRange, page, pageSize int) (provider.EventsPage, error) {
	f.evCalls++
	return provider.EventsPage{Page: page, PageSize: pageSize}, nil
}

func TestSyncSummaryOnlyProvider(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	prov := &fakeProvider{
		id:   provider.MiniMaxID,
		caps: provider.Capabilities{Summary: true, UsageEvents: false},
	}
	svc := New(prov, st)
	if err := svc.Run(context.Background(), "auto"); err != nil {
		t.Fatal(err)
	}
	if prov.sumCalls != 1 {
		t.Fatalf("sumCalls=%d", prov.sumCalls)
	}
	if prov.evCalls != 0 {
		t.Fatalf("evCalls=%d", prov.evCalls)
	}
	v, ok, _ := st.GetState(context.Background(), provider.MiniMaxID, "last_successful_sync_at")
	if !ok || v == "" {
		t.Fatal("expected last_successful_sync_at")
	}
	_, ok, _ = st.GetState(context.Background(), provider.MiniMaxID, "last_full_sync_year")
	if ok {
		t.Fatal("summary-only provider should not set last_full_sync_year")
	}
}

func TestSyncSummaryOnlyProviderFetchFails(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	prov := &fakeProvider{
		id:     provider.MiniMaxID,
		caps:   provider.Capabilities{Summary: true, UsageEvents: false},
		sumErr: errors.New("fetch failed"),
	}
	svc := New(prov, st)
	if err := svc.Run(context.Background(), "auto"); err == nil {
		t.Fatal("expected error")
	}
	_, ok, _ := st.GetState(context.Background(), provider.MiniMaxID, "last_successful_sync_at")
	if ok {
		t.Fatal("should not write last_successful_sync_at on failure")
	}
}
