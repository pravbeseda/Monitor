package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // the CGO-free driver of ADR 0005
)

// timeLayout is fixed-width, so stored timestamps sort lexicographically.
const timeLayout = "2006-01-02T15:04:05.000Z"

const schema = `
CREATE TABLE IF NOT EXISTS nodes (
	node           TEXT PRIMARY KEY,
	last_seen      TEXT NOT NULL,
	agent_version  TEXT NOT NULL,
	config_version TEXT NOT NULL,
	manifest       TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS measurements (
	node   TEXT NOT NULL,
	metric TEXT NOT NULL,
	labels TEXT NOT NULL,
	ts     TEXT NOT NULL,
	value  REAL NOT NULL,
	PRIMARY KEY (node, metric, labels, ts)
) WITHOUT ROWID;
`

// SQLite is the Storage implementation the hub runs on.
type SQLite struct {
	db *sql.DB
}

var _ Storage = (*SQLite)(nil)

// OpenSQLite opens the database at path, creating it and its schema when absent.
func OpenSQLite(path string) (*SQLite, error) {
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create schema in %s: %w", path, err)
	}
	return &SQLite{db: db}, nil
}

// SaveIngest stores one request in a single transaction; a failure stores nothing.
func (s *SQLite) SaveIngest(ctx context.Context, in Ingest) (err error) {
	manifest, err := json.Marshal(in.Manifest)
	if err != nil {
		return fmt.Errorf("encode manifest of %s: %w", in.Node, err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO nodes (node, last_seen, agent_version, config_version, manifest)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(node) DO UPDATE SET
			last_seen      = excluded.last_seen,
			agent_version  = excluded.agent_version,
			config_version = excluded.config_version,
			manifest       = excluded.manifest`,
		in.Node, formatTime(in.ReceivedAt), in.AgentVersion, in.ConfigVersion, string(manifest))
	if err != nil {
		return fmt.Errorf("save node %s: %w", in.Node, err)
	}

	for _, m := range in.Measurements {
		var labels string
		if labels, err = encodeLabels(m.Labels); err != nil {
			return fmt.Errorf("measurement %s of %s: %w", m.Metric, in.Node, err)
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO measurements (node, metric, labels, ts, value)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT DO NOTHING`,
			in.Node, m.Metric, labels, formatTime(m.TS), m.Value)
		if err != nil {
			return fmt.Errorf("save measurement %s of %s: %w", m.Metric, in.Node, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit ingest of %s: %w", in.Node, err)
	}
	return nil
}

// Close releases the database handle.
func (s *SQLite) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("close database: %w", err)
	}
	return nil
}

func formatTime(t time.Time) string {
	return t.UTC().Format(timeLayout)
}

func parseTime(s string) (time.Time, error) {
	t, err := time.Parse(timeLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse timestamp %q: %w", s, err)
	}
	return t, nil
}

// encodeLabels is the uniqueness key's label part: json.Marshal sorts map keys, so the
// same labels always encode to the same string.
func encodeLabels(labels map[string]string) (string, error) {
	if len(labels) == 0 {
		return "{}", nil
	}
	b, err := json.Marshal(labels)
	if err != nil {
		return "", fmt.Errorf("encode labels: %w", err)
	}
	return string(b), nil
}
