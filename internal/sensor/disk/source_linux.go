package disk

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// mountTable is where the kernel publishes what is mounted right now.
const mountTable = "/proc/self/mounts"

// systemSource reads the real mount table through /proc and statfs(2).
type systemSource struct{}

// System returns the mount source of the machine the agent runs on.
func System() Source { return systemSource{} }

func (systemSource) Mounts() ([]Mount, error) {
	table, err := os.Open(mountTable)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", mountTable, err)
	}
	defer func() { _ = table.Close() }()

	var mounts []Mount
	lines := bufio.NewScanner(table)
	for lines.Scan() {
		fields := strings.Fields(lines.Text())
		if len(fields) < 3 {
			continue
		}
		mounts = append(mounts, Mount{
			Path:      unescape(fields[1]),
			FS:        fields[2],
			Removable: removable(fields[0]),
		})
	}
	if err := lines.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", mountTable, err)
	}
	return mounts, nil
}

func (systemSource) Usage(mount string) (Usage, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(mount, &stat); err != nil {
		return Usage{}, fmt.Errorf("read %s: %w", mount, err)
	}
	size := uint64(stat.Bsize)
	return Usage{TotalBytes: stat.Blocks * size, AvailBytes: stat.Bavail * size}, nil
}

// removable asks the kernel, not the mount point: an internal disk mounted under /mnt is
// not removable. A partition carries the flag on its parent disk.
func removable(device string) bool {
	name, found := strings.CutPrefix(device, "/dev/")
	if !found {
		return false
	}
	block, err := filepath.EvalSymlinks(filepath.Join("/sys/class/block", name))
	if err != nil {
		return false
	}
	for _, path := range []string{block, filepath.Dir(block)} {
		if flag, err := os.ReadFile(filepath.Join(path, "removable")); err == nil {
			return strings.TrimSpace(string(flag)) == "1"
		}
	}
	return false
}

// unescape turns the octal escapes of /proc/self/mounts back into their characters, so a
// mount point with a space keeps the name the OS gave it.
func unescape(path string) string {
	if !strings.Contains(path, `\`) {
		return path
	}
	var out strings.Builder
	for i := 0; i < len(path); i++ {
		if path[i] == '\\' && i+3 < len(path) {
			if code, err := strconv.ParseUint(path[i+1:i+4], 8, 8); err == nil {
				out.WriteByte(byte(code))
				i += 3
				continue
			}
		}
		out.WriteByte(path[i])
	}
	return out.String()
}
