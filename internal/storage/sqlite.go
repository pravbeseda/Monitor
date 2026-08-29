package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // the CGO-free driver of ADR 0005
)

// timeLayout is fixed-width, so stored timestamps sort lexicographically. It also sets
// the resolution of the uniqueness key: one millisecond (docs/specs/ingest.md).
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

// States reads what the web page shows: one entry per node, latest value per series.
func (s *SQLite) States(ctx context.Context) ([]NodeState, error) {
	states, order, err := s.nodeStates(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.attachLatestValues(ctx, states); err != nil {
		return nil, err
	}

	out := make([]NodeState, 0, len(order))
	for _, node := range order {
		out = append(out, *states[node])
	}
	return out, nil
}

func (s *SQLite) nodeStates(ctx context.Context) (map[string]*NodeState, []string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT node, last_seen FROM nodes ORDER BY node`)
	if err != nil {
		return nil, nil, fmt.Errorf("read nodes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	states := map[string]*NodeState{}
	var order []string
	for rows.Next() {
		var node, lastSeen string
		if err := rows.Scan(&node, &lastSeen); err != nil {
			return nil, nil, fmt.Errorf("read nodes: %w", err)
		}
		seen, err := parseTime(lastSeen)
		if err != nil {
			return nil, nil, fmt.Errorf("node %s: %w", node, err)
		}
		states[node] = &NodeState{Node: node, LastSeen: seen}
		order = append(order, node)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("read nodes: %w", err)
	}
	return states, order, nil
}

// attachLatestValues keeps one row per series: the newest ts wins.
func (s *SQLite) attachLatestValues(ctx context.Context, states map[string]*NodeState) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT node, metric, labels, ts, value FROM (
			SELECT node, metric, labels, ts, value,
			       ROW_NUMBER() OVER (PARTITION BY node, metric, labels ORDER BY ts DESC) AS recency
			FROM measurements)
		WHERE recency = 1
		ORDER BY node, metric, labels`)
	if err != nil {
		return fmt.Errorf("read measurements: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var node, metric, labels, ts string
		var value Value
		if err := rows.Scan(&node, &metric, &labels, &ts, &value.Value); err != nil {
			return fmt.Errorf("read measurements: %w", err)
		}
		state, known := states[node]
		if !known {
			continue
		}
		value.Metric = metric
		if err := json.Unmarshal([]byte(labels), &value.Labels); err != nil {
			return fmt.Errorf("node %s: decode labels: %w", node, err)
		}
		if value.TS, err = parseTime(ts); err != nil {
			return fmt.Errorf("node %s: %w", node, err)
		}
		state.Values = append(state.Values, value)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read measurements: %w", err)
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
