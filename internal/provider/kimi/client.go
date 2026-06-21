package kimi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/godbobo/kiage/internal/config"
	"github.com/godbobo/kiage/internal/provider"
)

const (
	baseURL    = "https://api.kimi.com/coding/v1"
	usagesPath = "/usages"
	userAgent  = "kiage/1.0"
)

type Client struct {
	apiKey  string
	loc     *time.Location
	http    *http.Client
	baseURL string
}

func New(cfg config.Config) (*Client, error) {
	loc, err := cfg.Location()
	if err != nil {
		return nil, err
	}
	return &Client{
		apiKey:  cfg.Kimi.APIKey,
		loc:     loc,
		http:    &http.Client{Timeout: 60 * time.Second},
		baseURL: baseURL,
	}, nil
}

func NewWithHTTP(apiKey string, loc *time.Location, httpClient *http.Client, base string) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	if base == "" {
		base = baseURL
	}
	return &Client{apiKey: apiKey, loc: loc, http: httpClient, baseURL: base}
}

func (c *Client) ID() string { return provider.KimiID }

func (c *Client) DisplayName() string { return "Kimi" }

func (c *Client) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		Summary: true, UsageEvents: false, BillingCycle: false, SupportsCost: false,
	}
}

func (c *Client) Timezone() *time.Location { return c.loc }

func (c *Client) ValidateCredentials(ctx context.Context) error {
	if c.apiKey == "" {
		return provider.ErrInvalidCredential
	}
	_, err := c.FetchSummary(ctx)
	return err
}

func (c *Client) FetchSummary(ctx context.Context) (provider.Summary, error) {
	if c.apiKey == "" {
		return provider.Summary{}, provider.ErrInvalidCredential
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+usagesPath, nil)
	if err != nil {
		return provider.Summary{}, err
	}
	c.applyHeaders(req)

	resp, err := c.http.Do(req)
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
		return provider.Summary{}, fmt.Errorf("%w: usages status %d", provider.ErrProviderUnavailable, resp.StatusCode)
	}

	return parseSummary(body, string(body), c.loc)
}

func (c *Client) FetchUsageEvents(ctx context.Context, rng provider.DateRange, page, pageSize int) (provider.EventsPage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 100
	}
	return provider.EventsPage{Page: page, PageSize: pageSize}, nil
}

func (c *Client) applyHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
}

func parseSummary(body []byte, rawJSON string, loc *time.Location) (provider.Summary, error) {
	var payload struct {
		Usage  json.RawMessage `json:"usage"`
		Limits json.RawMessage `json:"limits"`
		User   struct {
			Membership struct {
				Level string `json:"level"`
			} `json:"membership"`
		} `json:"user"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return provider.Summary{}, fmt.Errorf("%w: %v", provider.ErrSchemaChanged, err)
	}

	s := provider.Summary{RawJSON: rawJSON}
	if level := payload.User.Membership.Level; level != "" {
		s.PlanName = planNameFromLevel(level)
		s.MembershipType = level
	}
	if s.PlanName == "" {
		s.PlanName = "Kimi Code"
	}

	var bars []provider.QuotaBar
	if payload.Limits != nil {
		var limits []struct {
			Window struct {
				Duration int    `json:"duration"`
				TimeUnit string `json:"timeUnit"`
			} `json:"window"`
			Detail json.RawMessage `json:"detail"`
		}
		if err := json.Unmarshal(payload.Limits, &limits); err != nil {
			return provider.Summary{}, fmt.Errorf("%w: limits: %v", provider.ErrSchemaChanged, err)
		}
		for _, item := range limits {
			if item.Window.Duration != 300 || !strings.EqualFold(item.Window.TimeUnit, "TIME_UNIT_MINUTE") {
				continue
			}
			bar, ok, err := quotaBarFromDetail(item.Detail, provider.LabelIntervalQuota, loc)
			if err != nil {
				return provider.Summary{}, err
			}
			if ok {
				bars = append(bars, bar)
			}
			break
		}
	}

	if payload.Usage != nil {
		bar, ok, err := quotaBarFromDetail(payload.Usage, provider.LabelWeeklyQuota, loc)
		if err != nil {
			return provider.Summary{}, err
		}
		if ok {
			bars = append(bars, bar)
			s.ResetAt = bar.ResetAt
		}
	}

	s.Bars = bars
	if len(bars) > 0 {
		s.TotalPercent = bars[0].Percent
	}
	return s, nil
}

func quotaBarFromDetail(raw json.RawMessage, label string, loc *time.Location) (provider.QuotaBar, bool, error) {
	var detail struct {
		Limit     json.RawMessage `json:"limit"`
		Used      json.RawMessage `json:"used"`
		Remaining json.RawMessage `json:"remaining"`
		ResetTime string          `json:"resetTime"`
	}
	if err := json.Unmarshal(raw, &detail); err != nil {
		return provider.QuotaBar{}, false, fmt.Errorf("%w: detail: %v", provider.ErrSchemaChanged, err)
	}

	limit, okLimit := parseAmount(detail.Limit)
	used, okUsed := parseAmount(detail.Used)
	remaining, okRemaining := parseAmount(detail.Remaining)
	if !okLimit {
		return provider.QuotaBar{}, false, nil
	}
	if !okUsed {
		if !okRemaining {
			return provider.QuotaBar{}, false, nil
		}
		used = limit - remaining
	}

	bar := provider.QuotaBar{
		Label:   label,
		Percent: usedPercent(used, limit),
	}
	if detail.ResetTime != "" {
		if t, err := time.Parse(time.RFC3339Nano, detail.ResetTime); err == nil {
			bar.ResetAt = t.In(loc)
		} else if t, err := time.Parse(time.RFC3339, detail.ResetTime); err == nil {
			bar.ResetAt = t.In(loc)
		}
	}
	return bar, true, nil
}

func parseAmount(raw json.RawMessage) (float64, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return f, true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		s = strings.TrimSpace(s)
		if s == "" {
			return 0, false
		}
		parsed, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	}
	return 0, false
}

func usedPercent(used, limit float64) float64 {
	if limit <= 0 {
		return 0
	}
	p := used / limit * 100
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return math.Round(p*10) / 10
}

func planNameFromLevel(level string) string {
	switch strings.ToUpper(level) {
	case "LEVEL_ANDANTE":
		return "Andante"
	case "LEVEL_INTERMEDIATE", "LEVEL_MODERATO":
		return "Moderato"
	case "LEVEL_ALLEGRETTO":
		return "Allegretto"
	default:
		if level == "" {
			return ""
		}
		name := strings.TrimPrefix(strings.ToUpper(level), "LEVEL_")
		if name == "" {
			return level
		}
		return strings.ToUpper(name[:1]) + strings.ToLower(name[1:])
	}
}
