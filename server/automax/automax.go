// Package automax automatically sets GOMEMLIMIT & GOMAXPROCS to match the linux container memory & cpu quotas, if any.
package automax

// config is used for tests.
type config struct {
	memCgroupV1 string
	memCgroupV2 string
	cpuCgroupV2 string
}

// SetMem puts GOMEMLIMIT to match the Linux container memory quota (if any), returning an undo function.
// It is a no-op in environments without a configured memory quota.
//
// The optional argument c is only used for test purposes.
func SetMem(c ...config) func() {
	return setMem(c...)
}

// SetCpu puts GOMAXPROCS to match the Linux container cpu quota (if any), returning an undo function.
// It is a no-op in environments without a configured cpu quota.
//
// The optional argument c is only used for test purposes.
func SetCpu(c ...config) func() {
	return setCpu(c...)
}
