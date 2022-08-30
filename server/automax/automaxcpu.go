package automax

import (
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
)

const (
	cgroupv2FilePath               = "/sys/fs/cgroup/cpu.max"
	cgroupV2CPUMaxDefaultCfsPeriod = 100_000
	cgroupV2CPUMaxQuotaMax         = "max"
	minGOMAXPROCS                  = 1
)

// It is a result of `cpu.cfs_quota_us / cpu.cfs_period_us`.
func setCpu(c ...config) func() {
	path := cgroupv2FilePath
	if len(c) > 0 {
		// we are running under tests.
		path = c[0].cpuCgroupV2
	}

	prev := currentMaxProcs()
	undo := func() {
		runtime.GOMAXPROCS(prev)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return undo
	}

	fields := strings.Fields(string(content))
	if len(fields) == 0 || len(fields) > 2 {
		// invalid format.
		return undo
	}
	if fields[0] == cgroupV2CPUMaxQuotaMax {
		// we should use all of cpus.
		return undo
	}

	cfs_quota_us, err := strconv.Atoi(fields[0])
	if err != nil {
		return undo
	}

	var cfs_period_us int
	if len(fields) == 1 {
		cfs_period_us = cgroupV2CPUMaxDefaultCfsPeriod
	} else {
		cfs_period_us, err = strconv.Atoi(fields[1])
		if err != nil {
			return undo
		}
	}

	quota := float64(cfs_quota_us) / float64(cfs_period_us)
	maxProcs := int(math.Floor(quota))
	if maxProcs < minGOMAXPROCS {
		maxProcs = minGOMAXPROCS
	}

	runtime.GOMAXPROCS(maxProcs)

	return undo
}

func currentMaxProcs() int {
	return runtime.GOMAXPROCS(0)
}
