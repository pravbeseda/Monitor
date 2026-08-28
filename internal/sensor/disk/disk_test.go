package disk_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pravbeseda/Monitor/internal/sensor"
	"github.com/pravbeseda/Monitor/internal/sensor/disk"
)

var collected = time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)

// fake is a mount table the test controls: usage is looked up by mount point, and a
// mount with no usage stands for one that vanished or cannot be read.
type fake struct {
	mounts []disk.Mount
	usage  map[string]disk.Usage
	err    error
}

func (f fake) Mounts() ([]disk.Mount, error) { return f.mounts, f.err }

func (f fake) Usage(mount string) (disk.Usage, error) {
	usage, ok := f.usage[mount]
	if !ok {
		return disk.Usage{}, errors.New("no such volume")
	}
	return usage, nil
}

func collect(t *testing.T, source disk.Source, allowed ...string) []sensor.Measurement {
	t.Helper()
	s := disk.New(source, func() []string { return allowed }, func() time.Time { return collected })
	got, err := s.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	return got
}

func byMetric(ms []sensor.Measurement, metric string) []sensor.Measurement {
	var out []sensor.Measurement
	for _, m := range ms {
		if m.Metric == metric {
			out = append(out, m)
		}
	}
	return out
}

var internalRoot = fake{
	mounts: []disk.Mount{{Path: "/", FS: "apfs"}},
	usage:  map[string]disk.Usage{"/": {TotalBytes: 1000, AvailBytes: 250}},
}

// spec: disk-sensor.md#enumeration
func TestCollectsAllowedVolume(t *testing.T) {
	got := collect(t, internalRoot, "apfs")

	if len(got) != 2 {
		t.Fatalf("measurements = %+v, want free_bytes and free_pct", got)
	}
	bytes := byMetric(got, "disk.free_bytes")
	pct := byMetric(got, "disk.free_pct")
	if len(bytes) != 1 || bytes[0].Value != 250 {
		t.Errorf("free_bytes = %+v, want the available bytes", bytes)
	}
	if len(pct) != 1 || pct[0].Value != 25 {
		t.Errorf("free_pct = %+v, want 25", pct)
	}
	if !bytes[0].TS.Equal(collected) {
		t.Errorf("ts = %v, want the collection time", bytes[0].TS)
	}
}

// spec: disk-sensor.md#enumeration — a type outside the allow-list is skipped.
func TestSkipsFilesystemOutsideAllowList(t *testing.T) {
	source := fake{
		mounts: []disk.Mount{{Path: "/", FS: "apfs"}, {Path: "/dev", FS: "devfs"}},
		usage:  map[string]disk.Usage{"/": {TotalBytes: 1000, AvailBytes: 250}, "/dev": {TotalBytes: 10, AvailBytes: 5}},
	}

	for _, m := range collect(t, source, "apfs") {
		if m.Labels["mount"] == "/dev" {
			t.Errorf("collected %+v, want devfs skipped", m)
		}
	}
}

// spec: disk-sensor.md#enumeration — a volume of zero total blocks has no percentage.
func TestSkipsEmptyVolume(t *testing.T) {
	source := fake{
		mounts: []disk.Mount{{Path: "/empty", FS: "apfs"}},
		usage:  map[string]disk.Usage{"/empty": {TotalBytes: 0, AvailBytes: 0}},
	}

	if got := collect(t, source, "apfs"); len(got) != 0 {
		t.Errorf("measurements = %+v, want none for a volume of no size", got)
	}
}

// spec: disk-sensor.md#enumeration — one unreadable volume never costs the others.
func TestKeepsCollectingAfterAnUnreadableVolume(t *testing.T) {
	source := fake{
		mounts: []disk.Mount{{Path: "/gone", FS: "apfs"}, {Path: "/", FS: "apfs"}},
		usage:  map[string]disk.Usage{"/": {TotalBytes: 1000, AvailBytes: 250}},
	}

	got := collect(t, source, "apfs")

	if len(got) != 2 {
		t.Fatalf("measurements = %+v, want the readable volume collected", got)
	}
	for _, m := range got {
		if m.Labels["mount"] != "/" {
			t.Errorf("collected %+v, want only the readable volume", m)
		}
	}
}

// spec: disk-sensor.md#enumeration — the mount table itself failing is an error.
func TestReportsUnreadableMountTable(t *testing.T) {
	s := disk.New(fake{err: errors.New("no mount table")},
		func() []string { return []string{"apfs"} }, time.Now)

	got, err := s.Collect(context.Background())

	if err == nil {
		t.Fatalf("Collect returned %+v, want an error", got)
	}
	if len(got) != 0 {
		t.Errorf("measurements = %+v, want none", got)
	}
}

// spec: disk-sensor.md#enumeration — nothing allowed means nothing collected.
func TestCollectsNothingWhenNoVolumeIsAllowed(t *testing.T) {
	if got := collect(t, internalRoot, "ext4"); len(got) != 0 {
		t.Errorf("measurements = %+v, want none", got)
	}
}

// spec: disk-sensor.md#labels
func TestLabelsIdentifyTheVolume(t *testing.T) {
	source := fake{
		mounts: []disk.Mount{{Path: "/Volumes/backup-a", FS: "APFS", Removable: true}},
		usage:  map[string]disk.Usage{"/Volumes/backup-a": {TotalBytes: 2000, AvailBytes: 1000}},
	}

	for _, m := range collect(t, source, "apfs") {
		want := map[string]string{"mount": "/Volumes/backup-a", "fs": "apfs", "removable": "true"}
		for label, value := range want {
			if m.Labels[label] != value {
				t.Errorf("%s label %s = %q, want %q", m.Metric, label, m.Labels[label], value)
			}
		}
		if len(m.Labels) != len(want) {
			t.Errorf("%s labels = %v, want exactly %v", m.Metric, m.Labels, want)
		}
	}
}

// spec: disk-sensor.md#labels — an internal volume says so rather than omitting the label.
func TestInternalVolumeIsLabelledNotRemovable(t *testing.T) {
	got := collect(t, internalRoot, "apfs")

	if got[0].Labels["removable"] != "false" {
		t.Errorf("removable = %q, want \"false\"", got[0].Labels["removable"])
	}
}

// spec: disk-sensor.md#applicability
func TestSensorIsApplicableEverywhere(t *testing.T) {
	s := disk.New(internalRoot, func() []string { return nil }, time.Now)

	if s.Name() != "disk" || !s.Applicable() {
		t.Errorf("manifest entry = %q applicable=%v, want disk applicable", s.Name(), s.Applicable())
	}
}

func TestFreePercentIsRoundedToTwoDecimals(t *testing.T) {
	source := fake{
		mounts: []disk.Mount{{Path: "/", FS: "ext4"}},
		usage:  map[string]disk.Usage{"/": {TotalBytes: 3000, AvailBytes: 1000}},
	}

	pct := byMetric(collect(t, source, "ext4"), "disk.free_pct")

	if len(pct) != 1 || pct[0].Value != 33.33 {
		t.Errorf("free_pct = %+v, want 33.33", pct)
	}
}
