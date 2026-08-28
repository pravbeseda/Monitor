// Package disk collects how much space each mounted volume has left
// (docs/specs/disk-sensor.md).
package disk

import (
	"context"
	"fmt"
	"maps"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/pravbeseda/Monitor/internal/sensor"
)

const (
	metricFreeBytes = "disk.free_bytes"
	metricFreePct   = "disk.free_pct"
)

// Mount is one entry of the machine's mount table.
type Mount struct {
	Path      string
	FS        string
	Removable bool
}

// Usage is what one volume reports about its size. Available excludes the blocks a
// filesystem reserves for root: they are not space the machine can use.
type Usage struct {
	TotalBytes uint64
	AvailBytes uint64
}

// Source is the platform-specific half of the sensor.
type Source interface {
	Mounts() ([]Mount, error)
	Usage(mount string) (Usage, error)
}

// Sensor reads free space per volume. The allow-list comes from the hub and changes
// while the agent runs, so it is read through a function rather than copied once.
type Sensor struct {
	source  Source
	allowed func() []string
	now     func() time.Time
}

var _ sensor.Sensor = (*Sensor)(nil)

// New builds the sensor over a mount source, the current filesystem allow-list and the
// agent's clock.
func New(source Source, allowed func() []string, now func() time.Time) *Sensor {
	return &Sensor{source: source, allowed: allowed, now: now}
}

// Name is the sensor id the configuration and the manifest use.
func (s *Sensor) Name() string { return "disk" }

// Applicable is true everywhere: every machine has at least one volume.
func (s *Sensor) Applicable() bool { return true }

// Collect returns what it could read: a volume that vanished or refused to answer costs
// only itself.
func (s *Sensor) Collect(context.Context) ([]sensor.Measurement, error) {
	mounts, err := s.source.Mounts()
	if err != nil {
		return nil, fmt.Errorf("read the mount table: %w", err)
	}

	allowed := make(map[string]bool)
	for _, fs := range s.allowed() {
		allowed[strings.ToLower(fs)] = true
	}

	at := s.now()
	var out []sensor.Measurement
	for _, mount := range mounts {
		fs := strings.ToLower(mount.FS)
		if !allowed[fs] {
			continue
		}
		usage, err := s.source.Usage(mount.Path)
		if err != nil || usage.TotalBytes == 0 {
			continue
		}
		labels := map[string]string{
			"mount":     mount.Path,
			"fs":        fs,
			"removable": strconv.FormatBool(mount.Removable),
		}
		free := float64(usage.AvailBytes)
		percent := round2(free / float64(usage.TotalBytes) * 100)
		out = append(out,
			measurement(metricFreeBytes, labels, free, at),
			measurement(metricFreePct, labels, percent, at))
	}
	return out, nil
}

func measurement(metric string, labels map[string]string, value float64, at time.Time) sensor.Measurement {
	return sensor.Measurement{
		Metric: metric,
		Labels: maps.Clone(labels),
		Value:  value,
		TS:     at,
	}
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}
