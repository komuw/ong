package errors

import (
	"io/fs"
	"os"
	"testing"

	"go.akshayshah.org/attest"
)

func TestStdLib(t *testing.T) {
	t.Parallel()

	t.Run("stdlib pass throughs", func(t *testing.T) {
		t.Parallel()

		err := prepFile()
		var targetErr *fs.PathError

		_, ok := err.(*stackError)
		attest.True(t, ok)
		attest.True(t, As(err, &targetErr))
		attest.True(t, Is(err, os.ErrNotExist))
		attest.NotZero(t, Unwrap(err))
	})
}
