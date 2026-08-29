// Package storage persists what the hub receives: measurements and node state.
package storage

import (
	"context"
	"time"
)

// Measurement is one reading of one metric, as it was collected.
type Measurement struct {
	Metric string
	Labels map[string]string
	Value  float64
	TS     time.Time
}

// SensorStatus is one line of an agent's manifest: a sensor the build contains and
// whether it applies to that machine.
type SensorStatus struct {
	Sensor     string
	Applicable bool
}

// Ingest is one accepted request, ready to be stored.
type Ingest struct {
	Node          string
	AgentVersion  string
	ConfigVersion string
	// ReceivedAt is hub time: the agent's clock never sets last-seen.
	ReceivedAt   time.Time
	Manifest     []SensorStatus
	Measurements []Measurement
}

// NodeState is what a node looks like right now: when it was last heard from, and the
// latest value of every series it reports.
type NodeState struct {
	Node     string
	LastSeen time.Time
	Values   []Value
}

// Value is the latest reading of one series.
type Value struct {
	Metric string
	Labels map[string]string
	Value  float64
	TS     time.Time
}

// Storage is the hub's persistence boundary (ADR 0005): SQLite behind an interface.
type Storage interface {
	// SaveIngest stores one request atomically — measurements, manifest and last-seen —
	// skipping measurements already stored under the same node, metric, labels and ts.
	SaveIngest(ctx context.Context, in Ingest) error
	// States returns every known node, ordered by name, with the latest value of each
	// of its series.
	States(ctx context.Context) ([]NodeState, error)
	Close() error
}
