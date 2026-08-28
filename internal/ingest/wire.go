package ingest

import (
	"strings"
	"time"

	"github.com/pravbeseda/Monitor/internal/config"
)

// The wire format of docs/specs/ingest.md. Unknown fields are ignored, so an older hub
// accepts a newer agent's request.

type request struct {
	Node          string         `json:"node"`
	AgentVersion  string         `json:"agent_version"`
	ConfigVersion string         `json:"config_version"`
	TS            string         `json:"ts"`
	Manifest      []sensorStatus `json:"manifest"`
	// A pointer tells an absent list from an empty one: empty is valid, absent is not.
	Measurements *[]measurement `json:"measurements"`
}

type sensorStatus struct {
	Sensor     string `json:"sensor"`
	Applicable bool   `json:"applicable"`
}

type measurement struct {
	Metric string            `json:"metric"`
	Labels map[string]string `json:"labels"`
	Value  *float64          `json:"value"`
	TS     string            `json:"ts"`
}

// response carries the configuration only when the agent's version differs from the
// hub's; otherwise both fields are omitted and the body is {}.
type response struct {
	ConfigVersion string       `json:"config_version,omitempty"`
	Config        *agentConfig `json:"config,omitempty"`
}

type agentConfig struct {
	BaseTick    string                  `json:"base_tick"`
	Filesystems []string                `json:"filesystems"`
	Sensors     map[string]sensorConfig `json:"sensors"`
}

type sensorConfig struct {
	Enabled  bool   `json:"enabled"`
	Interval string `json:"interval"`
}

type errorBody struct {
	Error string `json:"error"`
}

func deliver(agent config.Agent) *agentConfig {
	out := &agentConfig{
		BaseTick:    formatDuration(agent.BaseTick),
		Filesystems: agent.Filesystems,
		Sensors:     make(map[string]sensorConfig, len(agent.Sensors)),
	}
	for name, sensor := range agent.Sensors {
		out.Sensors[name] = sensorConfig{
			Enabled:  sensor.Enabled,
			Interval: formatDuration(sensor.Interval),
		}
	}
	return out
}

// formatDuration writes what an operator would write: 5m rather than 5m0s.
func formatDuration(d time.Duration) string {
	text := d.String()
	if strings.HasSuffix(text, "m0s") {
		text = strings.TrimSuffix(text, "0s")
	}
	if strings.HasSuffix(text, "h0m") {
		text = strings.TrimSuffix(text, "0m")
	}
	return text
}
