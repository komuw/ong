package automax

import (
	"fmt"
	"io"
	"os"
	"testing"

	"go.akshayshah.org/attest"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	// call flag.Parse() here if TestMain uses flags
	goleak.VerifyTestMain(m)
}

func TestSetMem(t *testing.T) { //nolint:tparallel
	t.Parallel()
	// This tests can run in parallel with others but not with themselves.

	dir := t.TempDir()

	f1, err := os.CreateTemp(dir, "pattern")
	attest.Ok(t, err)
	t.Cleanup(func() {
		f1.Close()
	})

	f2, err := os.CreateTemp(dir, "pattern")
	attest.Ok(t, err)
	t.Cleanup(func() {
		f2.Close()
	})

	cgroupV1Value := 125 * 1024 * 1024 // 125 MB
	_, err = io.WriteString(f1, fmt.Sprintln(cgroupV1Value))
	attest.Ok(t, err)

	cgroupV2Value := 456 * 1024 * 1024 // 456 MB
	_, err = io.WriteString(f2, fmt.Sprint(cgroupV2Value))
	attest.Ok(t, err)

	t.Run("cgroupV1", func(t *testing.T) {
		expected := int64(117964800)
		attest.NotEqual(t, currentMaxMem(), expected)

		c := []config{
			{memCgroupV1: f1.Name()},
		}
		undo := SetMem(c...)

		attest.Equal(t, currentMaxMem(), expected)
		undo()
	})

	t.Run("cgroupV2", func(t *testing.T) {
		expected := int64(430335590)
		attest.NotEqual(t, currentMaxMem(), expected)

		c := []config{
			{memCgroupV2: f2.Name()},
		}
		undo := SetMem(c...)

		attest.Equal(t, currentMaxMem(), expected)
		undo()
	})

	t.Run("cgroupV1 and cgroupV2", func(t *testing.T) {
		// cgroupV2 takes precedence.
		expected := int64(430335590)
		attest.NotEqual(t, currentMaxMem(), expected)

		c := []config{
			{
				memCgroupV1: f1.Name(),
				memCgroupV2: f2.Name(),
			},
		}
		undo := SetMem(c...)

		attest.Equal(t, currentMaxMem(), expected)
		undo()
	})

	t.Run("undo", func(t *testing.T) {
		expected := int64(430335590)
		attest.NotEqual(t, currentMaxMem(), expected)

		c := []config{
			{memCgroupV2: f2.Name()},
		}
		undo := SetMem(c...)

		attest.Equal(t, currentMaxMem(), expected)

		undo()
		attest.NotEqual(t, currentMaxMem(), expected)
	})
}
