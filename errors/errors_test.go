package errors

import (
	stdErrors "errors"
	"fmt"
	"io/fs"
	"os"
	"testing"

	"go.akshayshah.org/attest"
	"go.uber.org/goleak"
)

func hello() error {
	l := 90
	_ = l
	return bar()
}

func bar() error {
	okay := "okay"
	yello := okay
	err := foo()
	blue := yello
	_ = blue
	return err
}

func foo() error {
	return New("error in foo")
}

func prepFile() error {
	kk := "police"
	_ = kk
	if err := open("/tmp/nonExistentFile-akJGdadE.txt"); err != nil {
		return err
	}
	return nil
}

func open(p string) error {
	f, err := os.Open(p)
	if err != nil {
		return Wrap(err)
	}
	defer f.Close()

	return nil
}

func lateWrapping() error {
	return Wrap(hello())
}

func TestMain(m *testing.M) {
	// call flag.Parse() here if TestMain uses flags
	goleak.VerifyTestMain(m)
}

func TestStackError(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		t.Run("errors.New", func(t *testing.T) {
			t.Parallel()

			err := hello()

			sterr, ok := err.(*stackError)
			attest.True(t, ok)
			attest.Equal(t, sterr.Error(), "error in foo")

			stackTrace := sterr.getStackTrace()
			for _, v := range []string{
				"ong/errors/errors_test.go:30",
				"ong/errors/errors_test.go:23",
				"ong/errors/errors_test.go:17",
				"ong/errors/errors_test.go:70",
			} {
				attest.Subsequence(t, stackTrace, v, attest.Sprintf("\n\t%s: not found in stackTrace: %s", v, stackTrace))
			}
		})

		t.Run("errors.Wrap", func(t *testing.T) {
			t.Parallel()

			err := prepFile()

			sterr, ok := err.(*stackError)
			attest.True(t, ok)
			attest.True(t, stdErrors.Is(err, os.ErrNotExist))

			stackTrace := sterr.getStackTrace()
			for _, v := range []string{
				"ong/errors/errors_test.go:45",
				"ong/errors/errors_test.go:36",
				"ong/errors/errors_test.go:90",
			} {
				attest.Subsequence(t, stackTrace, v, attest.Sprintf("\n\t%s: not found in stackTrace: %s", v, stackTrace))
			}
		})

		t.Run("errors.Dwrap", func(t *testing.T) {
			t.Parallel()

			openFile := func(p string) (errp error) {
				defer Dwrap(&errp)

				f, err := os.Open(p)
				if err != nil {
					return err
				}
				defer f.Close()

				return nil
			}

			err := openFile("/tmp/nonExistentFile-akJGdadE.txt")

			sterr, ok := err.(*stackError)
			attest.True(t, ok)
			{
				// Is, As, Unwrap
				var targetErr *fs.PathError
				attest.True(t, stdErrors.Is(err, os.ErrNotExist))
				attest.NotZero(t, stdErrors.Unwrap(err))
				attest.True(t, stdErrors.As(err, &targetErr))
			}

			stackTrace := sterr.getStackTrace()
			for _, v := range []string{
				"ong/errors/errors_test.go:114",
				"ong/errors/errors_test.go:121",
			} {
				attest.Subsequence(t, stackTrace, v, attest.Sprintf("\n\t%s: not found in stackTrace: %s", v, stackTrace))
			}
		})

		t.Run("late wrapping does not affect traces", func(t *testing.T) {
			t.Parallel()

			err := lateWrapping()

			sterr, ok := err.(*stackError)
			attest.True(t, ok)

			stackTrace := sterr.getStackTrace()
			for _, v := range []string{
				"ong/errors/errors_test.go:30",
				"ong/errors/errors_test.go:23",
				"ong/errors/errors_test.go:17",
				"ong/errors/errors_test.go:53",
			} {
				attest.Subsequence(t, stackTrace, v, attest.Sprintf("\n\t%s: not found in stackTrace: %s", v, stackTrace))
			}
		})
	})

	t.Run("formattting", func(t *testing.T) {
		t.Parallel()

		err := hello()

		attest.Equal(t, fmt.Sprintf("%s", err), "error in foo") //nolint:gocritic
		attest.Equal(t, fmt.Sprintf("%q", err), `"error in foo"`)
		attest.Equal(t, fmt.Sprintf("%v", err), "error in foo") //nolint:gocritic

		extendedFormatting := fmt.Sprintf("%+v", err)
		for _, v := range []string{
			"ong/errors/errors_test.go:30",
			"ong/errors/errors_test.go:23",
			"ong/errors/errors_test.go:17",
			"ong/errors/errors_test.go:165",
		} {
			attest.Subsequence(t, extendedFormatting, v, attest.Sprintf("\n\t%s: not found in extendedFormatting: %s", v, extendedFormatting))
		}
	})

	t.Run("errors Is As Unwrap", func(t *testing.T) {
		t.Parallel()

		err := prepFile()
		var targetErr *fs.PathError

		_, ok := err.(*stackError)
		attest.True(t, ok)
		attest.True(t, stdErrors.Is(err, os.ErrNotExist))
		attest.NotZero(t, stdErrors.Unwrap(err))
		attest.True(t, stdErrors.As(err, &targetErr))

		_ = wrap(err, 2) // This is here to quiet golangci-lint which complains that wrap is always called with an argument of 3.
	})

	t.Run("multiple wrapping preserves traces", func(t *testing.T) {
		t.Parallel()

		f := func() (err error) {
			defer Dwrap(&err)

			e1 := New("hey")
			e2 := Wrap(e1)
			e3 := Errorf("fmting: %w", e2)

			return e3
		}

		err := f()
		extendedFormatting := fmt.Sprintf("%+v", err)

		attest.True(t, stdErrors.Is(err, &stackError{}))
		attest.Equal(t, err.Error(), "fmting: hey")
		for _, v := range []string{
			"ong/errors/errors_test.go:203",
			"ong/errors/errors_test.go:210",
		} {
			attest.Subsequence(t, extendedFormatting, v, attest.Sprintf("\n\t%s: not found in extendedFormatting: %s", v, extendedFormatting))
		}
	})
}
