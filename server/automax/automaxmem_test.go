package automax

import (
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/akshayjshah/attest"
)

func TestSetMem(t *testing.T) {
	t.Parallel()

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
	_, err = io.WriteString(f1, fmt.Sprint(cgroupV1Value))
	attest.Ok(t, err)

	cgroupV2Value := 456 * 1024 * 1024 // 456 MB
	_, err = io.WriteString(f2, fmt.Sprint(cgroupV2Value))
	attest.Ok(t, err)

	t.Run("cgroupV1", func(t *testing.T) {
		t.Parallel()

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
		t.Parallel()

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
		t.Parallel()

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
		t.Parallel()

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
