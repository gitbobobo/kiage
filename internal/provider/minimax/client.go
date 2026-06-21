package minimax

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	"github.com/godbobo/kiage/internal/config"
	"github.com/godbobo/kiage/internal/provider"
)

const (
	baseURL     = "https://api.minimaxi.com"
	remainsPath = "/v1/token_plan/remains"
	planName    = "Token Plan"
	userAgent   = "kiage/1.0"

	quotaStatusActive = 1
	quotaStatusNA     = 3
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
		apiKey:  cfg.MiniMax.APIKey,
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

func (c *Client) ID() string { return provider.MiniMaxID }

func (c *Client) DisplayName() string { return "MiniMax" }

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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+remainsPath, nil)
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
		return provider.Summary{}, fmt.Errorf("%w: remains status %d", provider.ErrProviderUnavailable, resp.StatusCode)
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

type modelRemain struct {
	ModelName                       string  `json:"model_name"`
	EndTime                         int64   `json:"end_time"`
	WeeklyEndTime                   int64   `json:"weekly_end_time"`
	CurrentIntervalStatus           int     `json:"current_interval_status"`
	CurrentWeeklyStatus             int     `json:"current_weekly_status"`
	CurrentIntervalRemainingPercent float64 `json:"current_interval_remaining_percent"`
	CurrentWeeklyRemainingPercent   float64 `json:"current_weekly_remaining_percent"`
}

func parseSummary(body []byte, rawJSON string, loc *time.Location) (provider.Summary, error) {
	var envelope struct {
		ModelRemains []modelRemain `json:"model_remains"`
		BaseResp     struct {
			StatusCode int    `json:"status_code"`
			StatusMsg  string `json:"status_msg"`
		} `json:"base_resp"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return provider.Summary{}, fmt.Errorf("%w: %v", provider.ErrSchemaChanged, err)
	}
	if code := envelope.BaseResp.StatusCode; code != 0 {
		if code == 1004 || code == 2049 {
			return provider.Summary{}, provider.ErrInvalidCredential
		}
		msg := envelope.BaseResp.StatusMsg
		if msg == "" {
			msg = fmt.Sprintf("status_code=%d", code)
		}
		return provider.Summary{}, fmt.Errorf("%w: %s", provider.ErrProviderUnavailable, msg)
	}

	s := provider.Summary{
		PlanName: planName,
		RawJSON:  rawJSON,
	}
	if len(envelope.ModelRemains) == 0 {
		return s, nil
	}

	var general *modelRemain
	for i := range envelope.ModelRemains {
		if envelope.ModelRemains[i].ModelName == "general" {
			general = &envelope.ModelRemains[i]
			break
		}
	}
	if general == nil {
		return provider.Summary{}, fmt.Errorf("%w: model_remains missing general", provider.ErrSchemaChanged)
	}

	intervalActive := general.CurrentIntervalStatus == quotaStatusActive
	weeklyActive := general.CurrentWeeklyStatus == quotaStatusActive

	if intervalActive && general.EndTime > 0 {
		s.ResetAt = time.UnixMilli(general.EndTime).In(loc)
		s.MembershipType = "interval"
	} else if weeklyActive && general.WeeklyEndTime > 0 {
		s.ResetAt = time.UnixMilli(general.WeeklyEndTime).In(loc)
		s.MembershipType = "weekly"
	}

	var bars []provider.QuotaBar
	if intervalActive {
		bars = append(bars, provider.QuotaBar{
			Label:   provider.LabelIntervalQuota,
			Percent: usedPercent(general.CurrentIntervalRemainingPercent),
		})
	}
	if weeklyActive {
		bars = append(bars, provider.QuotaBar{
			Label:   provider.LabelWeeklyQuota,
			Percent: usedPercent(general.CurrentWeeklyRemainingPercent),
		})
	}
	s.Bars = bars
	if len(s.Bars) > 0 {
		s.TotalPercent = s.Bars[0].Percent
	}
	return s, nil
}

func usedPercent(remaining float64) float64 {
	p := 100 - remaining
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return math.Round(p*10) / 10
}
