package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	"github.com/godbobo/kiage/internal/provider"
)

const GlobalProviderID = "__global__"

const (
	GlobalKeyLastSyncAt  = "last_global_sync_at"
	GlobalKeyLastBatchAt = "last_batch_at"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA synchronous=NORMAL; PRAGMA busy_timeout=5000;`); err != nil {
		db.Close()
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS schema_meta (version INTEGER NOT NULL)`,
		`INSERT INTO schema_meta(version) SELECT 1 WHERE NOT EXISTS (SELECT 1 FROM schema_meta)`,
		`CREATE TABLE IF NOT EXISTS usage_events (
			provider_id TEXT NOT NULL,
			event_id TEXT NOT NULL,
			timestamp_utc TEXT NOT NULL,
			local_date TEXT NOT NULL,
			model TEXT NOT NULL,
			input_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			total_tokens INTEGER NOT NULL DEFAULT 0,
			cost_cents REAL NOT NULL DEFAULT 0,
			checksum TEXT NOT NULL DEFAULT '',
			raw_json TEXT,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (provider_id, event_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_events_provider_date ON usage_events(provider_id, local_date)`,
		`CREATE TABLE IF NOT EXISTS daily_rollup (
			provider_id TEXT NOT NULL,
			local_date TEXT NOT NULL,
			total_tokens INTEGER NOT NULL DEFAULT 0,
			total_cost_cents REAL NOT NULL DEFAULT 0,
			event_count INTEGER NOT NULL DEFAULT 0,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (provider_id, local_date)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_daily_rollup_date ON daily_rollup(local_date)`,
		`CREATE TABLE IF NOT EXISTS summary_snapshot (
			provider_id TEXT PRIMARY KEY,
			plan_name TEXT,
			membership_type TEXT,
			billing_cycle_start TEXT,
			billing_cycle_end TEXT,
			reset_at TEXT,
			total_percent REAL,
			composer_percent REAL,
			api_percent REAL,
			raw_json TEXT,
			fetched_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS sync_state (
			provider_id TEXT NOT NULL,
			key TEXT NOT NULL,
			value TEXT NOT NULL,
			PRIMARY KEY (provider_id, key)
		)`,
		`CREATE TABLE IF NOT EXISTS sync_run (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			provider_id TEXT NOT NULL,
			status TEXT NOT NULL,
			mode TEXT NOT NULL,
			started_at TEXT NOT NULL,
			finished_at TEXT,
			last_date TEXT,
			error TEXT
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	return s.ensureBarsJSONColumn()
}

func (s *Store) ensureBarsJSONColumn() error {
	rows, err := s.db.Query(`PRAGMA table_info(summary_snapshot)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == "bars_json" {
			return rows.Err()
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.Exec(`ALTER TABLE summary_snapshot ADD COLUMN bars_json TEXT`)
	return err
}

func (s *Store) IntegrityCheck(ctx context.Context) error {
	var result string
	if err := s.db.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return fmt.Errorf("integrity check failed: %s", result)
	}
	return nil
}

func (s *Store) GetState(ctx context.Context, providerID, key string) (string, bool, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM sync_state WHERE provider_id=? AND key=?`, providerID, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	return v, err == nil, err
}

func (s *Store) SetState(ctx context.Context, providerID, key, value string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sync_state(provider_id, key, value) VALUES(?,?,?)
		ON CONFLICT(provider_id, key) DO UPDATE SET value=excluded.value`,
		providerID, key, value)
	return err
}

func (s *Store) GetGlobalState(ctx context.Context, key string) (string, bool, error) {
	return s.GetState(ctx, GlobalProviderID, key)
}

func (s *Store) SetGlobalState(ctx context.Context, key, value string) error {
	return s.SetState(ctx, GlobalProviderID, key, value)
}

func (s *Store) UpsertEvent(ctx context.Context, providerID string, ev provider.UsageEvent, checksum string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO usage_events(provider_id, event_id, timestamp_utc, local_date, model,
			input_tokens, output_tokens, total_tokens, cost_cents, checksum, raw_json, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(provider_id, event_id) DO UPDATE SET
			timestamp_utc=excluded.timestamp_utc,
			local_date=excluded.local_date,
			model=excluded.model,
			input_tokens=excluded.input_tokens,
			output_tokens=excluded.output_tokens,
			total_tokens=excluded.total_tokens,
			cost_cents=excluded.cost_cents,
			checksum=excluded.checksum,
			raw_json=excluded.raw_json,
			updated_at=excluded.updated_at
		WHERE excluded.checksum != usage_events.checksum`,
		providerID, ev.EventID, ev.Timestamp.UTC().Format(time.RFC3339), ev.LocalDate, ev.Model,
		ev.InputTokens, ev.OutputTokens, ev.TotalTokens, ev.CostCents, checksum, ev.RawJSON, now)
	return err
}

func (s *Store) RebuildDailyRollup(ctx context.Context, providerID, localDate string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO daily_rollup(provider_id, local_date, total_tokens, total_cost_cents, event_count, updated_at)
		SELECT ?, ?, COALESCE(SUM(total_tokens),0), COALESCE(SUM(cost_cents),0), COUNT(*), ?
		FROM usage_events WHERE provider_id=? AND local_date=?
		ON CONFLICT(provider_id, local_date) DO UPDATE SET
			total_tokens=excluded.total_tokens,
			total_cost_cents=excluded.total_cost_cents,
			event_count=excluded.event_count,
			updated_at=excluded.updated_at`,
		providerID, localDate, now, providerID, localDate)
	return err
}

type DailyRollup struct {
	Date        string
	TotalTokens int64
	TotalCost   float64
	EventCount  int
}

func (s *Store) ListDailyRollup(ctx context.Context, providerID string, from, to string) ([]DailyRollup, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT local_date, total_tokens, total_cost_cents, event_count
		FROM daily_rollup WHERE provider_id=? AND local_date>=? AND local_date<=?
		ORDER BY local_date`, providerID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DailyRollup
	for rows.Next() {
		var d DailyRollup
		if err := rows.Scan(&d.Date, &d.TotalTokens, &d.TotalCost, &d.EventCount); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

type SummaryRow struct {
	PlanName        string
	MembershipType  string
	BillingStart    time.Time
	BillingEnd      time.Time
	ResetAt         time.Time
	TotalPercent    float64
	ComposerPercent float64
	APIPercent      float64
	Bars            []provider.QuotaBar
	FetchedAt       time.Time
}

func (s *Store) SaveSummary(ctx context.Context, providerID string, sum provider.Summary) error {
	bars := sum.Bars
	if len(bars) == 0 && providerID == provider.CursorID {
		bars = provider.LegacyBarsFromSummary(sum)
	}
	barsJSON, err := json.Marshal(bars)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO summary_snapshot(provider_id, plan_name, membership_type,
			billing_cycle_start, billing_cycle_end, reset_at,
			total_percent, composer_percent, api_percent, bars_json, raw_json, fetched_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(provider_id) DO UPDATE SET
			plan_name=excluded.plan_name,
			membership_type=excluded.membership_type,
			billing_cycle_start=excluded.billing_cycle_start,
			billing_cycle_end=excluded.billing_cycle_end,
			reset_at=excluded.reset_at,
			total_percent=excluded.total_percent,
			composer_percent=excluded.composer_percent,
			api_percent=excluded.api_percent,
			bars_json=excluded.bars_json,
			raw_json=excluded.raw_json,
			fetched_at=excluded.fetched_at`,
		providerID, sum.PlanName, sum.MembershipType,
		sum.BillingCycleStart.UTC().Format(time.RFC3339),
		sum.BillingCycleEnd.UTC().Format(time.RFC3339),
		sum.ResetAt.UTC().Format(time.RFC3339),
		sum.TotalPercent, sum.ComposerPercent, sum.APIPercent,
		string(barsJSON), sum.RawJSON, time.Now().UTC().Format(time.RFC3339))
	return err
}

func (s *Store) LoadSummary(ctx context.Context, providerID string) (SummaryRow, bool, error) {
	var row SummaryRow
	var bs, be, ra, fa string
	var barsJSON sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT plan_name, membership_type, billing_cycle_start, billing_cycle_end, reset_at,
			total_percent, composer_percent, api_percent, bars_json, fetched_at
		FROM summary_snapshot WHERE provider_id=?`, providerID).Scan(
		&row.PlanName, &row.MembershipType, &bs, &be, &ra,
		&row.TotalPercent, &row.ComposerPercent, &row.APIPercent, &barsJSON, &fa)
	if err == sql.ErrNoRows {
		return row, false, nil
	}
	if err != nil {
		return row, false, err
	}
	row.BillingStart, _ = time.Parse(time.RFC3339, bs)
	row.BillingEnd, _ = time.Parse(time.RFC3339, be)
	row.ResetAt, _ = time.Parse(time.RFC3339, ra)
	row.FetchedAt, _ = time.Parse(time.RFC3339, fa)
	if barsJSON.Valid && barsJSON.String != "" {
		_ = json.Unmarshal([]byte(barsJSON.String), &row.Bars)
	}
	if len(row.Bars) == 0 && providerID == provider.CursorID {
		row.Bars = provider.CursorBarsFromPercents(row.TotalPercent, row.ComposerPercent, row.APIPercent)
	}
	return row, true, nil
}

func (s *Store) SumRollupRange(ctx context.Context, providerID, from, to string) (tokens int64, cost float64, err error) {
	err = s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(total_tokens),0), COALESCE(SUM(total_cost_cents),0)
		FROM daily_rollup WHERE provider_id=? AND local_date>=? AND local_date<=?`,
		providerID, from, to).Scan(&tokens, &cost)
	return
}

func (s *Store) DeleteEventsBefore(ctx context.Context, providerID, date string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM usage_events WHERE provider_id=? AND local_date<?`, providerID, date)
	return err
}

func (s *Store) Vacuum(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `VACUUM`)
	return err
}
