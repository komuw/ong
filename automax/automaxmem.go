package automax

import (
	"math"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
)

const (
	procSelfCgroup  = "/proc/self/cgroup"
	cgroupRoot      = "/sys/fs/cgroup"
	ignoreLimit     = 10 * 1024 * 1024  // 10MB
	noLimit         = math.MaxInt64 / 2 // cgroup v1 uses a number close to math.MaxInt64 to mean that there is no limit.
	memLimitPercent = (80.0 / 100.0)    // set GOMEMLIMIT to 80% of cgroup's memory limit
)

func setMem(c ...config) func() {
	procCgroup, root := procSelfCgroup, cgroupRoot
	if len(c) > 0 {
		// we are running under tests.
		procCgroup, root = c[0].procSelfCgroup, c[0].cgroupRoot
	}

	prev := currentMaxMem()
	undo := func() {
		debug.SetMemoryLimit(prev)
	}

	if _, ok := os.LookupEnv("GOMEMLIMIT"); ok {
		// The user has chosen a value.
		return undo
	}

	n, ok := memLimit(procCgroup, root)
	if !ok {
		return undo
	}

	// set GOMEMLIMIT to memLimitPercent of cgroup's memory limit
	limit := int64(memLimitPercent * float64(n)) // limit in bytes.
	if limit < ignoreLimit {
		return undo
	}
	debug.SetMemoryLimit(limit)

	return undo
}

// memLimit returns the smallest memory limit of the cgroup of the current process and of its parent cgroups.
// procCgroup is the path of the `/proc/self/cgroup` file and root is the path where cgroups are mounted.
func memLimit(procCgroup, root string) (int64, bool) {
	content, err := os.ReadFile(procCgroup)
	if err != nil {
		return 0, false
	}

	var v1Path, v2Path string
	for line := range strings.Lines(string(content)) {
		// Each line has the format `hierarchy-ID:controller-list:cgroup-path`
		parts := strings.SplitN(strings.TrimSpace(line), ":", 3)
		if len(parts) != 3 {
			continue
		}
		if parts[0] == "0" && parts[1] == "" {
			v2Path = parts[2]
		}
		if slices.Contains(strings.Split(parts[1], ","), "memory") {
			v1Path = parts[2]
		}
	}

	top, dir, file := "", "", ""
	switch {
	case v1Path != "":
		// In hybrid mode, the memory controller is in cgroup v1. So v1 takes precedence.
		top = filepath.Join(root, "memory")
		dir, file = filepath.Join(top, v1Path), "memory.limit_in_bytes"
	case v2Path != "":
		top = filepath.Clean(root)
		dir, file = filepath.Join(top, v2Path), "memory.max"
	default:
		return 0, false
	}

	if dir != top && !strings.HasPrefix(dir, top+string(filepath.Separator)) {
		// The cgroup path is outside of the mount. This can happen in cgroup namespaces.
		dir = top
	}

	// The cgroup path in `/proc/self/cgroup` may not exist in the mount, eg in containers without a cgroup namespace.
	// Those paths are skipped.
	var lowest int64
	for ; ; dir = filepath.Dir(dir) {
		if n, ok := readLimit(filepath.Join(dir, file)); ok && (lowest == 0 || n < lowest) {
			lowest = n
		}
		if dir == top {
			break
		}
	}

	return lowest, lowest > 0
}

// readLimit returns the memory limit in the file at path, or false if there is no limit.
func readLimit(path string) (int64, bool) {
	content, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}

	// For cgroup v2, a value of `max` means no limit. It fails to parse.
	n, err := strconv.ParseInt(strings.TrimSpace(string(content)), 10, 64)
	if err != nil || n <= 0 || n >= noLimit {
		return 0, false
	}

	return n, true
}

func currentMaxMem() int64 {
	return debug.SetMemoryLimit(-6) // negative input allows retrieval of the currently set memory limit.
}
