package server

import (
	"os"
	"strings"
	"testing"
)

// Test taken from; https://github.com/golang/go/blob/go1.19beta1/src/os/rlimit_test.go
func Test_setRlimit(t *testing.T) {
	// we cannot use t.Parallel() in this tests since `t.Setenv()` does not allow it.
	t.Setenv("GOWEB_RUNNING_IN_TESTS", "TRUE")

	maxFiles := 65_536 * 2 // most OSes set the soft limit at 1024, on ubuntu22.04 in github actions it is 65_536

	t.Run("rlimit reached", func(t *testing.T) {
		var files []*os.File
		var errs []error

		for i := 0; i < maxFiles; i++ {
			f, err := os.Open("rlimit.go")
			if err != nil {
				errs = append(errs, err)
				break
			}
			files = append(files, f)
		}

		for _, f := range files {
			f.Close()
		}

		if len(errs) <= 0 {
			t.Error("expected rlimit errors")
		}
		if !strings.Contains(errs[0].Error(), "too many open files") {
			t.Error("expected rlimit error")
		}
	})

	t.Run("rlimit NOT reached", func(t *testing.T) {
		if os.Getenv("GITHUB_ACTIONS") != "" {
			// setRlimit() fails in github actions with error: `operation not permitted`
			// specifically the call to `unix.Setrlimit()`
			return
		}

		setRlimit()

		var files []*os.File
		var errs []error

		for i := 0; i < maxFiles; i++ {
			f, err := os.Open("rlimit.go")
			if err != nil {
				errs = append(errs, err)
				break
			}
			files = append(files, f)
		}

		for _, f := range files {
			f.Close()
		}

		if len(errs) > 0 {
			t.Logf("\n\t err: %v\n", errs[0])
			t.Error("did NOT expect rlimit errors")
		}
	})
}
