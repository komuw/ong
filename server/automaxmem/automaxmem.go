// Package automaxmem automatically sets GOMEMLIMIT to match the Linux
// container memory quota, if any.
package automaxmem

import (
	"os"
	"runtime/debug"
	"strconv"
)

const (
	cgroupV1    = "/sys/fs/cgroup/memory/memory.limit_in_bytes"
	cgroupv2    = "/sys/fs/cgroup/memory.max"
	ignoreLimit = 10 * 1024 * 1024 // 10MB
)

// config is used for tests.
type config struct {
	cgroupV1 string
	cgroupV2 string
}

// Set GOMEMLIMIT to match the Linux container memory quota (if any), returning an undo function.
// It is a no-op on non-Linux systems and in Linux environments without a configured memory quota.
//
// The optional argument c is only for test purposes.
func Set(c ...config) func() {
	prev := currentMaxMem()
	undo := func() {
		debug.SetMemoryLimit(prev)
	}

	var content []byte
	var err error
	if len(c) > 0 {
		// we are running under tests.
		content, err = os.ReadFile(c[0].cgroupV2)
		if err != nil {
			content, err = os.ReadFile(c[0].cgroupV1)
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

	n, err := strconv.ParseInt(string(content), 10, 64)
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
