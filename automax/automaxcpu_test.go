package automax

import (
	"io"
	"os"
	"sync"
	"testing"

	"go.akshayshah.org/attest"
)

func TestSetCpu(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex

	write := func(cgroupV2Value string) string {
		dir := t.TempDir()
		f, err := os.CreateTemp(dir, "pattern")
		attest.Ok(t, err)

		_, err = io.WriteString(f, cgroupV2Value)
		attest.Ok(t, err)

		t.Cleanup(func() {
			f.Close()
		})

		return f.Name()
	}

	t.Run("cgroupV2", func(t *testing.T) {
		t.Parallel()
		mu.Lock()
		defer mu.Unlock()

		/*
			use 3 cpus.
			docker run -it --entrypoint /bin/bash --cpus="3" redis
		*/
		fName := write("300000 100000")

		expected := int(3)
		attest.NotEqual(t, currentMaxProcs(), expected)

		c := []config{
			{cpuCgroupV2: fName},
		}
		undo := SetCpu(c...)

		attest.Equal(t, currentMaxProcs(), expected)
		undo()
	})

	t.Run("cgroup max", func(t *testing.T) {
		t.Parallel()
		mu.Lock()
		defer mu.Unlock()

		fName := write("max 100000")

		expected := currentMaxProcs()

		c := []config{
			{cpuCgroupV2: fName},
		}
		undo := SetCpu(c...)

		attest.Equal(t, currentMaxProcs(), expected)
		undo()
	})

	t.Run("cpu less than 1", func(t *testing.T) {
		t.Parallel()
		mu.Lock()
		defer mu.Unlock()

		/*
			use 50% of cpu.
			docker run -it --entrypoint /bin/bash --cpus=".5" redis
		*/
		fName := write("50000 100000")

		expected := minGOMAXPROCS
		attest.NotEqual(t, currentMaxProcs(), expected)

		c := []config{
			{cpuCgroupV2: fName},
		}
		undo := SetCpu(c...)

		attest.Equal(t, currentMaxProcs(), expected)
		undo()
	})

	t.Run("one field", func(t *testing.T) {
		t.Parallel()
		mu.Lock()
		defer mu.Unlock()

		fName := write("500000")

		expected := int(5)
		attest.NotEqual(t, currentMaxProcs(), expected)

		c := []config{
			{cpuCgroupV2: fName},
		}
		undo := SetCpu(c...)

		attest.Equal(t, currentMaxProcs(), expected)
		undo()
	})

	t.Run("corrupt file", func(t *testing.T) {
		t.Parallel()
		mu.Lock()
		defer mu.Unlock()

		fName := write("its corrupt")

		expected := currentMaxProcs()

		c := []config{
			{cpuCgroupV2: fName},
		}
		undo := SetCpu(c...)

		attest.Equal(t, currentMaxProcs(), expected)
		undo()
	})
}
