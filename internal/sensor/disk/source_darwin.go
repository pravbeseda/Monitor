package disk

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// systemSource reads the real mount table through statfs(2).
type systemSource struct{}

// System returns the mount source of the machine the agent runs on.
func System() Source { return systemSource{} }

func (systemSource) Mounts() ([]Mount, error) {
	count, err := unix.Getfsstat(nil, unix.MNT_NOWAIT)
	if err != nil {
		return nil, fmt.Errorf("count mounted volumes: %w", err)
	}
	stats := make([]unix.Statfs_t, count)
	count, err = unix.Getfsstat(stats, unix.MNT_NOWAIT)
	if err != nil {
		return nil, fmt.Errorf("list mounted volumes: %w", err)
	}

	mounts := make([]Mount, 0, count)
	for _, stat := range stats[:count] {
		mounts = append(mounts, Mount{
			Path:      unix.ByteSliceToString(stat.Mntonname[:]),
			FS:        unix.ByteSliceToString(stat.Fstypename[:]),
			Removable: stat.Flags&unix.MNT_REMOVABLE != 0,
		})
	}
	return mounts, nil
}

func (systemSource) Usage(mount string) (Usage, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(mount, &stat); err != nil {
		return Usage{}, fmt.Errorf("read %s: %w", mount, err)
	}
	size := uint64(stat.Bsize)
	usage := Usage{TotalBytes: stat.Blocks * size, AvailBytes: stat.Bavail * size}
	// What macOS calls available counts purgeable space; statfs does not (ADR 0014).
	if forImportantUsage, answered := available(mount); answered {
		usage.AvailBytes = forImportantUsage
	}
	return usage, nil
}
