package provider

import (
	"context"
	"errors"
	"time"
)

const CursorID = "cursor"

var (
	ErrInvalidCredential    = errors.New("invalid credential")
	ErrRateLimited          = errors.New("rate limited")
	ErrProviderUnavailable  = errors.New("provider unavailable")
	ErrSchemaChanged        = errors.New("schema changed")
)

type Capabilities struct {
	Summary       bool
	UsageEvents   bool
	BillingCycle  bool
}

type Summary struct {
	PlanName          string
	MembershipType    string
	BillingCycleStart time.Time
	BillingCycleEnd   time.Time
	ResetAt           time.Time
	TotalPercent      float64
	ComposerPercent   float64
	APIPercent        float64
	OnDemandEnabled   bool
	OnDemandUsedCents float64
	RawJSON           string
}

type UsageEvent struct {
	EventID      string
	Timestamp    time.Time
	LocalDate    string
	Model        string
	InputTokens  int64
	OutputTokens int64
	TotalTokens  int64
	CostCents    float64
	RawJSON      string
}

type DateRange struct {
	Start time.Time
	End   time.Time
}

type EventsPage struct {
	Events     []UsageEvent
	Page       int
	PageSize   int
	TotalCount int
	HasMore    bool
}

type Provider interface {
	ID() string
	DisplayName() string
	Capabilities() Capabilities
	Timezone() *time.Location
	ValidateCredentials(ctx context.Context) error
	FetchSummary(ctx context.Context) (Summary, error)
	FetchUsageEvents(ctx context.Context, rng DateRange, page, pageSize int) (EventsPage, error)
}
