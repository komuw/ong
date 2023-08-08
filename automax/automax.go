// Package automax automatically sets GOMEMLIMIT & GOMAXPROCS to match the linux container memory & cpu quotas, if any.
package automax

import "testing"

// config is used for tests.
type config struct {
	memCgroupV1 string
	memCgroupV2 string
	cpuCgroupV2 string
}

// SetMem puts GOMEMLIMIT to match the linux container memory quota (if any), returning an undo function.
//
// It is a no-op in environments without a configured memory quota.
//
// The optional argument c is only used for internal test purposes.
func SetMem(c ...config) func() {
	if len(c) > 0 && !testing.Testing() {
		panic("c should only be passed in as an argument from tests")
	}
	return setMem(c...)
}

// SetCpu puts GOMAXPROCS to match the linux container cpu quota (if any), returning an undo function.
//
// It is a no-op in environments without a configured cpu quota.
//
// The optional argument c is only used for internal test purposes.
func SetCpu(c ...config) func() {
	if len(c) > 0 && !testing.Testing() {
		panic("c should only be passed in as an argument from tests")
	}
	return setCpu(c...)
}
