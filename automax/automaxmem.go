package automax

import (
	"os"
	"runtime/debug"
	"strconv"
	"strings"
)

const (
	cgroupV1    = "/sys/fs/cgroup/memory/memory.limit_in_bytes"
	cgroupv2    = "/sys/fs/cgroup/memory.max"
	ignoreLimit = 10 * 1024 * 1024 // 10MB
)

func setMem(c ...config) func() {
	prev := currentMaxMem()
	undo := func() {
		debug.SetMemoryLimit(prev)
	}

	var content []byte
	var err error
	if len(c) > 0 {
		// we are running under tests.
		content, err = os.ReadFile(c[0].memCgroupV2)
		if err != nil {
			content, err = os.ReadFile(c[0].memCgroupV1)
		}
		if err != nil {
			return undo
		}
	} else {
		// start with v2 since it is the most recent and we expect most systems to have it.
		content, err = os.ReadFile(cgroupv2)
		if err != nil {
			content, err = os.ReadFile(cgroupV1)
		}
		if err != nil {
			return undo
		}
	}

	n, err := strconv.ParseInt(strings.TrimSpace(string(content)), 10, 64)
	if err != nil {
		return undo
	}

	// set GOMEMLIMIT to 90% of cgroup's memory limit
	limit := int64(0.9 * float64(n)) // limit in bytes.
	if limit < ignoreLimit {
		return undo
	}
	debug.SetMemoryLimit(limit)

	return undo
}

func currentMaxMem() int64 {
	return debug.SetMemoryLimit(-6) // negative input allows retrieval of the currently set memory limit.
}
