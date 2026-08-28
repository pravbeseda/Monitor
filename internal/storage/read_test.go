package storage

import (
	"encoding/json"
	"testing"
	"time"
)

// The read side arrives with the web page (stage 1, step 7); until then the tests read
// the database directly through these helpers.

type nodeRow struct {
	LastSeen      time.Time
	AgentVersion  string
	ConfigVersion string
	Manifest      []SensorStatus
}

func (s *SQLite) node(t *testing.T, name string) nodeRow {
	t.Helper()

	var lastSeen, manifest string
	var row nodeRow
	err := s.db.QueryRow(
		`SELECT last_seen, agent_version, config_version, manifest FROM nodes WHERE node = ?`,
		name,
	).Scan(&lastSeen, &row.AgentVersion, &row.ConfigVersion, &manifest)
	if err != nil {
		t.Fatalf("read node %s: %v", name, err)
	}
	if row.LastSeen, err = parseTime(lastSeen); err != nil {
		t.Fatalf("node %s: %v", name, err)
	}
	if err := json.Unmarshal([]byte(manifest), &row.Manifest); err != nil {
		t.Fatalf("decode manifest of %s: %v", name, err)
	}
	return row
}

func (s *SQLite) measurements(t *testing.T, node string) []Measurement {
	t.Helper()

	rows, err := s.db.Query(
		`SELECT metric, labels, ts, value FROM measurements WHERE node = ? ORDER BY metric, labels, ts`,
		node,
	)
	if err != nil {
		t.Fatalf("read measurements of %s: %v", node, err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			t.Errorf("close rows: %v", err)
		}
	}()

	var out []Measurement
	for rows.Next() {
		var m Measurement
		var labels, ts string
		if err := rows.Scan(&m.Metric, &labels, &ts, &m.Value); err != nil {
			t.Fatalf("scan measurement of %s: %v", node, err)
		}
		if err := json.Unmarshal([]byte(labels), &m.Labels); err != nil {
			t.Fatalf("decode labels of %s: %v", node, err)
		}
		if m.TS, err = parseTime(ts); err != nil {
			t.Fatalf("measurement of %s: %v", node, err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read measurements of %s: %v", node, err)
	}
	return out
}
