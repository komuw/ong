// Package automax automatically sets GOMEMLIMIT to match the linux cgroup memory limit, if any.
//
// It does not set GOMAXPROCS. The Go runtime already sets it from the cgroup cpu limit.
// See [runtime.GOMAXPROCS].
package automax

import "testing"

// config is used for tests.
type config struct {
	procSelfCgroup string
	cgroupRoot     string
}

// SetMem puts GOMEMLIMIT to match the linux cgroup memory limit (if any), returning an undo function.
//
// It uses the smallest memory limit of the cgroup of the current process and of its parent cgroups.
// It is a no-op if the GOMEMLIMIT environment variable is set, or in environments without a configured memory limit.
//
// The optional argument c is only used for internal test purposes.
func SetMem(c ...config) func() {
	if len(c) > 0 && !testing.Testing() {
		panic("optional argument c should only be used for internal test purposes")
	}

	return setMem(c...)
}
