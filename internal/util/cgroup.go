package util

import (
	"os"
	"strconv"
	"strings"
)

// cgroup v2 paths. Inside a container with its own cgroup namespace these are
// the container's own limit and usage; on a host they are absent, which is how
// callers tell the difference.
const (
	cgroupMemoryMaxPath     = "/sys/fs/cgroup/memory.max"
	cgroupMemoryCurrentPath = "/sys/fs/cgroup/memory.current"
)

// CgroupMemoryLimit returns the container's memory limit in bytes. It reports
// false when there is no limit: outside a container, under cgroup v1, or when
// the limit reads "max".
func CgroupMemoryLimit() (int64, bool) {
	return readCgroupBytes(cgroupMemoryMaxPath)
}

// CgroupMemoryUsage returns the container's current memory usage in bytes,
// reporting false when it cannot be read.
func CgroupMemoryUsage() (int64, bool) {
	return readCgroupBytes(cgroupMemoryCurrentPath)
}

func readCgroupBytes(path string) (int64, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	text := strings.TrimSpace(string(raw))
	if text == "max" {
		return 0, false
	}
	value, err := strconv.ParseInt(text, 10, 64)
	if err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}
