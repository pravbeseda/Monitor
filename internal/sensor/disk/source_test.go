package disk_test

import (
	"testing"

	"github.com/pravbeseda/Monitor/internal/sensor/disk"
)

// The platform source is thin but easy to break; this checks it against the machine the
// tests run on, whichever of the two supported platforms that is.
func TestSystemSourceReadsThisMachine(t *testing.T) {
	source := disk.System()

	mounts, err := source.Mounts()
	if err != nil {
		t.Fatalf("Mounts: %v", err)
	}
	if len(mounts) == 0 {
		t.Fatal("no mounts, want at least the root volume")
	}
	for _, mount := range mounts {
		if mount.Path == "" || mount.FS == "" {
			t.Errorf("mount = %+v, want a path and a filesystem type", mount)
		}
	}

	usage, err := source.Usage("/")
	if err != nil {
		t.Fatalf("Usage(/): %v", err)
	}
	if usage.TotalBytes == 0 || usage.AvailBytes > usage.TotalBytes {
		t.Errorf("usage = %+v, want a plausible root volume", usage)
	}
}
