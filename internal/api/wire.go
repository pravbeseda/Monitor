// Package api is the wire format of /api/v1/ingest, shared by the agent that writes it
// and the hub that reads it (docs/specs/ingest.md).
package api

import (
	"strings"
	"time"

	"github.com/pravbeseda/monitor/internal/config"
)

// The wire format of docs/specs/ingest.md. Unknown fields are ignored, so an older hub
// accepts a newer agent's request.

// Request is what an agent posts on every tick.
type Request struct {
	Node          string         `json:"node"`
	AgentVersion  string         `json:"agent_version"`
	ConfigVersion string         `json:"config_version"`
	TS            string         `json:"ts"`
	Manifest      []SensorStatus `json:"manifest"`
	// A pointer tells an absent list from an empty one: empty is valid, absent is not.
	Measurements *[]Measurement `json:"measurements"`
}

// SensorStatus is one line of the manifest: a sensor the build contains.
type SensorStatus struct {
	Sensor     string `json:"sensor"`
	Applicable bool   `json:"applicable"`
}

// Measurement is one reading on the wire.
type Measurement struct {
	Metric string            `json:"metric"`
	Labels map[string]string `json:"labels"`
	Value  *float64          `json:"value"`
	TS     string            `json:"ts"`
}

// Response carries the configuration only when the agent's version differs from the
// hub's; otherwise both fields are omitted and the body is {}.
type Response struct {
	ConfigVersion string       `json:"config_version,omitempty"`
	Config        *AgentConfig `json:"config,omitempty"`
}

// AgentConfig is the flat configuration the agent applies as it arrives.
type AgentConfig struct {
	BaseTick    string                  `json:"base_tick"`
	Filesystems []string                `json:"filesystems"`
	SkipMounts  []string                `json:"skip_mounts"`
	Sensors     map[string]SensorConfig `json:"sensors"`
}

// SensorConfig is one sensor's slot in that configuration.
type SensorConfig struct {
	Enabled  bool   `json:"enabled"`
	Interval string `json:"interval"`
}

// ErrorBody is what every refused request answers with.
type ErrorBody struct {
	Error string `json:"error"`
}

// Deliver flattens a resolved configuration onto the wire.
func Deliver(agent config.Agent) *AgentConfig {
	out := &AgentConfig{
		BaseTick:    FormatDuration(agent.BaseTick),
		Filesystems: agent.Filesystems,
		SkipMounts:  agent.SkipMounts,
		Sensors:     make(map[string]SensorConfig, len(agent.Sensors)),
	}
	for name, sensor := range agent.Sensors {
		out.Sensors[name] = SensorConfig{
			Enabled:  sensor.Enabled,
			Interval: FormatDuration(sensor.Interval),
		}
	}
	return out
}

// FormatDuration writes what an operator would write: 5m rather than 5m0s.
func FormatDuration(d time.Duration) string {
	text := d.String()
	if strings.HasSuffix(text, "m0s") {
		text = strings.TrimSuffix(text, "0s")
	}
	if strings.HasSuffix(text, "h0m") {
		text = strings.TrimSuffix(text, "0m")
	}
	return text
}
