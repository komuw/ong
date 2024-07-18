package errors

import (
	stdErrors "errors"
	"fmt"
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
			err := Errorf("yolo %s okay : %w", "one", e1)

			{
				attest.Subsequence(t, err.Error(), "hello")
				attest.Subsequence(t, err.Error(), "yolo")
				attest.Subsequence(t, err.Error(), "one")
			}
			{
				attest.Subsequence(t, fmt.Sprintf("%+#v", err), "hello")
				attest.Subsequence(t, fmt.Sprintf("%+#v", err), "yolo")
				attest.Subsequence(t, fmt.Sprintf("%+#v", err), "one")
				attest.Subsequence(t, fmt.Sprintf("%+#v", err), "ong/errors/stdlib_test.go:41")
			}
		}
	})

	t.Run("issues/463", func(t *testing.T) {
		t.Parallel()

		var errs []error
		for i := range 2 {
			e := New("err")
			errs = append(errs, Errorf("failed(%d): %w", i, e))
		}

		if err := Join(errs...); err != nil {
			fmt.Printf("%+#v", err)
		}
	})
}
