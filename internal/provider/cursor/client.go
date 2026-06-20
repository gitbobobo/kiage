package cursor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/godbobo/kiage/internal/config"
	"github.com/godbobo/kiage/internal/provider"
)

const (
	baseURL       = "https://cursor.com"
	usageSummary  = baseURL + "/api/usage-summary"
	usageEvents   = baseURL + "/api/dashboard/get-filtered-usage-events"
	userAgent     = "kiage/1.0"
	defaultPageSz = 100
)

type Client struct {
	token    string
	loc      *time.Location
	http     *http.Client
	baseURL  string
}

func New(cfg config.Config) (*Client, error) {
	loc, err := cfg.Location()
	if err != nil {
		return nil, err
	}
	return &Client{
		token:   cfg.Cursor.SessionToken,
		loc:     loc,
		http:    &http.Client{Timeout: 60 * time.Second},
		baseURL: baseURL,
	}, nil
}

func NewWithHTTP(token string, loc *time.Location, httpClient *http.Client, base string) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	if base == "" {
		base = baseURL
	}
	return &Client{token: token, loc: loc, http: httpClient, baseURL: base}
}

func (c *Client) ID() string { return provider.CursorID }

func (c *Client) DisplayName() string { return "Cursor" }

func (c *Client) Capabilities() provider.Capabilities {
	return provider.Capabilities{Summary: true, UsageEvents: true, BillingCycle: true}
}

func (c *Client) Timezone() *time.Location { return c.loc }

func (c *Client) ValidateCredentials(ctx context.Context) error {
	_, err := c.FetchSummary(ctx)
	return err
}

func (c *Client) FetchSummary(ctx context.Context) (provider.Summary, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/usage-summary", nil)
	if err != nil {
		return provider.Summary{}, err
	}
	c.applyHeaders(req, false)

	resp, err := c.doWithRetry(ctx, req)
	if err != nil {
		return provider.Summary{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return provider.Summary{}, err
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return provider.Summary{}, provider.ErrInvalidCredential
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return provider.Summary{}, provider.ErrRateLimited
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return provider.Summary{}, fmt.Errorf("%w: usage-summary status %d", provider.ErrProviderUnavailable, resp.StatusCode)
	}

	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return provider.Summary{}, fmt.Errorf("%w: %v", provider.ErrSchemaChanged, err)
	}

	summary, err := parseSummary(raw, string(body))
	if err != nil {
		return provider.Summary{}, err
	}
	return summary, nil
}

func (c *Client) FetchUsageEvents(ctx context.Context, rng provider.DateRange, page, pageSize int) (provider.EventsPage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = defaultPageSz
	}

	payload := map[string]any{
		"startDate": strconv.FormatInt(rng.Start.UnixMilli(), 10),
		"endDate":   strconv.FormatInt(rng.End.UnixMilli(), 10),
		"page":      page,
		"pageSize":  pageSize,
	}
	data, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/dashboard/get-filtered-usage-events", bytes.NewReader(data))
	if err != nil {
		return provider.EventsPage{}, err
	}
	c.applyHeaders(req, true)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doWithRetry(ctx, req)
	if err != nil {
		return provider.EventsPage{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return provider.EventsPage{}, err
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return provider.EventsPage{}, provider.ErrInvalidCredential
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return provider.EventsPage{}, provider.ErrRateLimited
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return provider.EventsPage{}, fmt.Errorf("%w: events status %d", provider.ErrProviderUnavailable, resp.StatusCode)
	}

	var raw struct {
		TotalUsageEventsCount int              `json:"totalUsageEventsCount"`
		UsageEventsDisplay  []json.RawMessage `json:"usageEventsDisplay"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return provider.EventsPage{}, fmt.Errorf("%w: %v", provider.ErrSchemaChanged, err)
	}

	events := make([]provider.UsageEvent, 0, len(raw.UsageEventsDisplay))
	for _, item := range raw.UsageEventsDisplay {
		ev, err := parseEvent(item, c.loc)
		if err != nil {
			continue
		}
		events = append(events, ev)
	}

	total := raw.TotalUsageEventsCount
	hasMore := page*pageSize < total
	return provider.EventsPage{
		Events:     events,
		Page:       page,
		PageSize:   pageSize,
		TotalCount: total,
		HasMore:    hasMore,
	}, nil
}

func (c *Client) applyHeaders(req *http.Request, post bool) {
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Cookie", "WorkosCursorSessionToken="+c.token)
	if post {
		req.Header.Set("Origin", "https://cursor.com")
	}
}

func (c *Client) doWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	backoff := 500 * time.Millisecond
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		reqClone := req.Clone(ctx)
		resp, err := c.http.Do(reqClone)
		if err != nil {
			lastErr = err
			time.Sleep(backoff)
			backoff = min(backoff*2, 30*time.Second)
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if sec, err := strconv.Atoi(ra); err == nil {
					time.Sleep(time.Duration(sec) * time.Second)
					continue
				}
			}
			time.Sleep(backoff)
			backoff = min(backoff*2, 30*time.Second)
			lastErr = provider.ErrRateLimited
			continue
		}
		return resp, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, provider.ErrProviderUnavailable
}

func parseSummary(raw map[string]any, rawJSON string) (provider.Summary, error) {
	s := provider.Summary{RawJSON: rawJSON}

	if v, ok := raw["membershipType"].(string); ok {
		s.MembershipType = v
		s.PlanName = strings.ToUpper(v[:1]) + v[1:]
	}
	if v, ok := raw["billingCycleStart"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			s.BillingCycleStart = t
		}
	}
	if v, ok := raw["billingCycleEnd"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			s.BillingCycleEnd = t
			s.ResetAt = t
		}
	}

	plan, _ := raw["individualUsage"].(map[string]any)
	if plan != nil {
		p, _ := plan["plan"].(map[string]any)
		if p != nil {
			s.TotalPercent = asFloat(p["totalPercentUsed"])
			s.ComposerPercent = asFloat(p["autoPercentUsed"])
			s.APIPercent = asFloat(p["apiPercentUsed"])
		}
		od, _ := plan["onDemand"].(map[string]any)
		if od != nil {
			s.OnDemandEnabled = asBool(od["enabled"])
			s.OnDemandUsedCents = asFloat(od["used"])
		}
	}
	return s, nil
}

func parseEvent(raw json.RawMessage, loc *time.Location) (provider.UsageEvent, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return provider.UsageEvent{}, err
	}

	tsStr, _ := m["timestamp"].(string)
	ms, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return provider.UsageEvent{}, err
	}
	ts := time.UnixMilli(ms).In(loc)

	var input, output int64
	var cost float64
	if tu, ok := m["tokenUsage"].(map[string]any); ok {
		input = int64(asFloat(tu["inputTokens"]))
		output = int64(asFloat(tu["outputTokens"]))
		if v := asFloat(tu["totalCents"]); v > 0 {
			cost = v
		}
	}
	if v := asFloat(m["chargedCents"]); v > 0 {
		cost = v
	}

	model, _ := m["model"].(string)
	total := input + output
	if cw, ok := m["tokenUsage"].(map[string]any); ok {
		total += int64(asFloat(cw["cacheWriteTokens"]))
		total += int64(asFloat(cw["cacheReadTokens"]))
	}

	ev := provider.UsageEvent{
		EventID:      eventID(raw),
		Timestamp:    ts,
		LocalDate:    ts.Format("2006-01-02"),
		Model:        model,
		InputTokens:  input,
		OutputTokens: output,
		TotalTokens:  total,
		CostCents:    cost,
		RawJSON:      string(raw),
	}
	return ev, nil
}

func eventID(raw json.RawMessage) string {
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:16])
}

func asFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case json.Number:
		f, _ := t.Float64()
		return f
	default:
		return 0
	}
}

func asBool(v any) bool {
	b, _ := v.(bool)
	return b
}

func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
