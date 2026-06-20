package glm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/godbobo/kiage/internal/config"
	"github.com/godbobo/kiage/internal/provider"
)

const (
	baseURL       = "https://open.bigmodel.cn"
	quotaPath     = "/api/monitor/usage/quota/limit"
	modelUsagePath = "/api/monitor/usage/model-usage"
	userAgent     = "kiage/1.0"
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
		apiKey:  cfg.GLM.APIKey,
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

func (c *Client) ID() string { return provider.GLMID }

func (c *Client) DisplayName() string { return "GLM" }

func (c *Client) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		Summary: true, UsageEvents: true, BillingCycle: true, SupportsCost: false,
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+quotaPath, nil)
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
		return provider.Summary{}, fmt.Errorf("%w: quota status %d", provider.ErrProviderUnavailable, resp.StatusCode)
	}

	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return provider.Summary{}, fmt.Errorf("%w: %v", provider.ErrSchemaChanged, err)
	}
	return parseSummary(envelope.Data, string(body), c.loc)
}

func (c *Client) FetchUsageEvents(ctx context.Context, rng provider.DateRange, page, pageSize int) (provider.EventsPage, error) {
	if c.apiKey == "" {
		return provider.EventsPage{}, provider.ErrInvalidCredential
	}
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 100
	}

	start := rng.Start.In(c.loc)
	end := rng.End.In(c.loc)
	q := url.Values{}
	q.Set("startTime", start.Format("2006-01-02 15:04:05"))
	q.Set("endTime", end.Format("2006-01-02 15:04:05"))

	reqURL := c.baseURL + modelUsagePath + "?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return provider.EventsPage{}, err
	}
	c.applyHeaders(req)

	resp, err := c.http.Do(req)
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
		return provider.EventsPage{}, fmt.Errorf("%w: model-usage status %d", provider.ErrProviderUnavailable, resp.StatusCode)
	}

	events, err := parseUsageEvents(body, c.loc)
	if err != nil {
		return provider.EventsPage{}, err
	}

	return provider.EventsPage{
		Events:     events,
		Page:       page,
		PageSize:   pageSize,
		TotalCount: len(events),
		HasMore:    false,
	}, nil
}

func (c *Client) applyHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
}

func parseSummary(raw json.RawMessage, rawJSON string, loc *time.Location) (provider.Summary, error) {
	var data struct {
		Level  string `json:"level"`
		Limits []struct {
			Type          string  `json:"type"`
			Unit          int     `json:"unit"`
			Number        int     `json:"number"`
			Percentage    float64 `json:"percentage"`
			NextResetTime int64   `json:"nextResetTime"`
		} `json:"limits"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return provider.Summary{}, fmt.Errorf("%w: %v", provider.ErrSchemaChanged, err)
	}

	now := time.Now().In(loc)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	monthEnd := monthStart.AddDate(0, 1, 0)

	s := provider.Summary{
		PlanName:          planLabel(data.Level),
		MembershipType:    data.Level,
		BillingCycleStart: monthStart,
		BillingCycleEnd:   monthEnd,
		RawJSON:           rawJSON,
	}

	var tokenBars []provider.QuotaBar
	for _, lim := range data.Limits {
		switch lim.Type {
		case "TOKENS_LIMIT":
			label := tokenLimitLabel(lim.Number, len(tokenBars))
			bar := provider.QuotaBar{
				Label:   label,
				Percent: lim.Percentage,
			}
			if lim.NextResetTime > 0 {
				bar.ResetAt = time.UnixMilli(lim.NextResetTime).In(loc)
			}
			tokenBars = append(tokenBars, bar)
		case "TIME_LIMIT":
			bar := provider.QuotaBar{
				Label:   "MCP 月度",
				Percent: lim.Percentage,
			}
			if lim.NextResetTime > 0 {
				bar.ResetAt = time.UnixMilli(lim.NextResetTime).In(loc)
			}
			s.Bars = append(s.Bars, bar)
		}
	}
	s.Bars = append(tokenBars, s.Bars...)
	if len(tokenBars) > 0 && !tokenBars[0].ResetAt.IsZero() {
		s.ResetAt = tokenBars[0].ResetAt
	} else if len(s.Bars) > 0 && !s.Bars[0].ResetAt.IsZero() {
		s.ResetAt = s.Bars[0].ResetAt
	}
	if len(s.Bars) > 0 {
		s.TotalPercent = s.Bars[0].Percent
	}
	if len(s.Bars) > 1 {
		s.ComposerPercent = s.Bars[1].Percent
	}
	if len(s.Bars) > 2 {
		s.APIPercent = s.Bars[2].Percent
	}
	return s, nil
}

func tokenLimitLabel(number, idx int) string {
	if number > 0 {
		return fmt.Sprintf("%d小时配额", number)
	}
	if idx == 1 {
		return "每周配额"
	}
	return "Token 配额"
}

func planLabel(level string) string {
	switch strings.ToLower(level) {
	case "lite":
		return "Lite"
	case "standard", "pro":
		return "Pro"
	case "max":
		return "Max"
	default:
		if level == "" {
			return "—"
		}
		return strings.ToUpper(level[:1]) + level[1:]
	}
}

func parseUsageEvents(body []byte, loc *time.Location) ([]provider.UsageEvent, error) {
	var envelope struct {
		Data struct {
			XTime         []string `json:"x_time"`
			TokensUsage   []int64  `json:"tokensUsage"`
			ModelDataList []struct {
				ModelName   string  `json:"modelName"`
				TokensUsage []int64 `json:"tokensUsage"`
			} `json:"modelDataList"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("%w: %v", provider.ErrSchemaChanged, err)
	}
	data := envelope.Data
	if len(data.XTime) == 0 {
		return nil, nil
	}

	var events []provider.UsageEvent
	appendHour := func(model string, hourIdx int, tokens int64) {
		if tokens <= 0 {
			return
		}
		ts, err := time.ParseInLocation("2006-01-02 15:04", data.XTime[hourIdx], loc)
		if err != nil {
			return
		}
		raw := fmt.Sprintf("%s|%s|%d", data.XTime[hourIdx], model, tokens)
		h := sha256.Sum256([]byte(raw))
		events = append(events, provider.UsageEvent{
			EventID:     hex.EncodeToString(h[:16]),
			Timestamp:   ts,
			LocalDate:   ts.Format("2006-01-02"),
			Model:       model,
			TotalTokens: tokens,
			RawJSON:     raw,
		})
	}

	if len(data.ModelDataList) > 0 {
		for _, m := range data.ModelDataList {
			for i, tok := range m.TokensUsage {
				if i >= len(data.XTime) {
					break
				}
				appendHour(m.ModelName, i, tok)
			}
		}
	} else {
		for i, tok := range data.TokensUsage {
			appendHour("", i, tok)
		}
	}
	return events, nil
}
