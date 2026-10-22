package automax

import (
	"os"
	"path/filepath"
	"testing"

	"go.akshayshah.org/attest"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	// call flag.Parse() here if TestMain uses flags
	goleak.VerifyTestMain(m)
}

// fakeCgroup creates a fake `/proc/self/cgroup` file and a fake cgroup mount.
// files maps a path relative to the cgroup mount to its content.
// If procSelfCgroup is empty, the `/proc/self/cgroup` file is not created.
func fakeCgroup(t *testing.T, procSelfCgroup string, files map[string]string) config {
	t.Helper()

	dir := t.TempDir()
	root := filepath.Join(dir, "sys", "fs", "cgroup")
	attest.Ok(t, os.MkdirAll(root, 0o755))

	proc := filepath.Join(dir, "cgroup")
	if procSelfCgroup != "" {
		attest.Ok(t, os.WriteFile(proc, []byte(procSelfCgroup), 0o644))
	}

	for name, content := range files {
		path := filepath.Join(root, name)
		attest.Ok(t, os.MkdirAll(filepath.Dir(path), 0o755))
		attest.Ok(t, os.WriteFile(path, []byte(content), 0o644))
	}

	return config{procSelfCgroup: proc, cgroupRoot: root}
}

func TestSetMem(t *testing.T) { //nolint:tparallel
	t.Parallel()
	// The subtests change a global setting. So they do not run in parallel with each other.

	const (
		v2Service = "0::/system.slice/app.service\n"
		mb125     = "131072000\n" // 125MB
		mb456     = "478150656\n" // 456MB
	)
	limitOf := func(n int64) int64 { return int64(memLimitPercent * float64(n)) }
	limit125 := limitOf(131072000)
	limit456 := limitOf(478150656)

	tests := []struct {
		name     string
		proc     string
		files    map[string]string
		expected int64 // zero means that the limit should not change.
	}{
		{
			name:     "cgroupV2 namespace root",
			proc:     "0::/\n",
			files:    map[string]string{"memory.max": mb456},
			expected: limit456,
		},
		{
			name:     "cgroupV2 limit on own cgroup",
			proc:     v2Service,
			files:    map[string]string{"system.slice/app.service/memory.max": mb456},
			expected: limit456,
		},
		{
			name: "cgroupV2 limit on parent cgroup",
			proc: v2Service,
			files: map[string]string{
				"system.slice/memory.max":             mb125,
				"system.slice/app.service/memory.max": "max\n",
			},
			expected: limit125,
		},
		{
			name: "cgroupV2 smallest limit is used",
			proc: v2Service,
			files: map[string]string{
				"system.slice/memory.max":             mb125,
				"system.slice/app.service/memory.max": mb456,
			},
			expected: limit125,
		},
		{
			name: "cgroupV2 no limit",
			proc: v2Service,
			files: map[string]string{
				"system.slice/memory.max":             "max\n",
				"system.slice/app.service/memory.max": "max\n",
			},
		},
		{
			name:     "cgroupV2 path outside of mount",
			proc:     "0::/../../app.service\n",
			files:    map[string]string{"memory.max": mb456},
			expected: limit456,
		},
		{
			name:     "cgroupV1",
			proc:     "12:memory:/system.slice/app.service\n11:cpu,cpuacct:/system.slice/app.service\n",
			files:    map[string]string{"memory/system.slice/app.service/memory.limit_in_bytes": mb125},
			expected: limit125,
		},
		{
			name:  "cgroupV1 no limit",
			proc:  "12:memory:/\n",
			files: map[string]string{"memory/memory.limit_in_bytes": "9223372036854771712\n"},
		},
		{
			name:     "cgroupV1 container without cgroup namespace",
			proc:     "12:memory:/docker/abc123\n",
			files:    map[string]string{"memory/memory.limit_in_bytes": mb125},
			expected: limit125,
		},
		{
			name: "hybrid uses cgroupV1",
			proc: "12:memory:/\n0::/\n",
			files: map[string]string{
				"memory/memory.limit_in_bytes": mb125,
				"memory.max":                   mb456,
			},
			expected: limit125,
		},
		{
			name:  "limit too small",
			proc:  "0::/\n",
			files: map[string]string{"memory.max": "1048576\n"},
		},
		{
			name: "no proc file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fakeCgroup(t, tt.proc, tt.files)
			prev := currentMaxMem()
			if tt.expected != 0 {
				attest.NotEqual(t, prev, tt.expected)
			}

			undo := SetMem(c)

			if tt.expected != 0 {
				attest.Equal(t, currentMaxMem(), tt.expected)
			} else {
				attest.Equal(t, currentMaxMem(), prev)
			}

			undo()
			attest.Equal(t, currentMaxMem(), prev)
		})
	}
}

func TestSetMemEnv(t *testing.T) {
	// t.Setenv cannot be used in parallel tests.
	t.Setenv("GOMEMLIMIT", "1GiB")

	c := fakeCgroup(t, "0::/\n", map[string]string{"memory.max": "478150656\n"})

	prev := currentMaxMem()
	undo := SetMem(c)
	attest.Equal(t, currentMaxMem(), prev)
	undo()
}
