package errors

import (
	stdErrors "errors"
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

	t.Run("stdlib errorf", func(t *testing.T) {
		t.Parallel()

		{
			e1 := New("hello")
			err := Errorf("yolo: %w", e1)
			attest.Subsequence(t, err.Error(), "hello")
			attest.Subsequence(t, err.Error(), "yolo")
		}

		{
			e1 := stdErrors.New("hello")
			err := Errorf("yolo: %w", e1)
			attest.Subsequence(t, err.Error(), "hello")
			attest.Subsequence(t, err.Error(), "yolo")
		}
	})
}
