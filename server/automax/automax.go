// Package automax automatically sets GOMEMLIMIT & GOMAXPROCS to match the Linux
// container memory quota, if any.
package automax

// SetMem puts GOMEMLIMIT to match the Linux container memory quota (if any), returning an undo function.
// It is a no-op on non-Linux systems and in Linux environments without a configured memory quota.
//
// The optional argument c is only used for test purposes.
func SetMem(c ...config) func() {
	return setMem(c...)
}
