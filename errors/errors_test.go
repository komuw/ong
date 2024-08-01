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

type apiError struct{ err error }

func (a apiError) Error() string { return a.err.Error() }

func (a apiError) Unwrap() error {
	// This method is the one that causes issue.
	// See; https://github.com/komuw/ong/issues/466
	return a.err
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
				"ong/errors/errors_test.go:80",
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
				"ong/errors/errors_test.go:100",
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
				"ong/errors/errors_test.go:124",
				"ong/errors/errors_test.go:131",
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
			"ong/errors/errors_test.go:175",
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
			"ong/errors/errors_test.go:213",
			"ong/errors/errors_test.go:220",
		} {
			attest.Subsequence(t, extendedFormatting, v, attest.Sprintf("\n\t%s: not found in extendedFormatting: %s", v, extendedFormatting))
		}
	})

	t.Run("issues/466", func(t *testing.T) {
		t.Parallel()

		{ // success
			var err error = apiError{err: New("hey")}
			err = Wrap(err)
			got := fmt.Sprintf("%+#v", err)

			attest.Subsequence(t, got, "hey")
			attest.Subsequence(t, got, "ong/errors/errors_test.go:237")
		}

		{ // nil
			var err error = apiError{err: New("hey")}
			err = nil
			err = Wrap(err)
			got := fmt.Sprintf("%+#v", err)

			attest.Subsequence(t, got, "nil")
		}
	})
}

func TestStackTrace(t *testing.T) {
	t.Parallel()

	t.Run("handles nil", func(t *testing.T) {
		t.Parallel()

		var err error = nil
		got := StackTrace(err)
		attest.Equal(t, got, "")
	})

	t.Run("traces", func(t *testing.T) {
		t.Parallel()

		{
			err := New("hello")
			got := StackTrace(err)
			attest.Subsequence(t, got, "ong/errors/errors_test.go:271")
		}
		{
			err := stdErrors.New("hello stdErrors")
			got := StackTrace(err)
			attest.Subsequence(t, got, "")
		}
		{
			e1 := New("hello")
			err := Wrap(e1)

			got := StackTrace(err)
			attest.Subsequence(t, got, "ong/errors/errors_test.go:281")
		}
		{
			e1 := New("hello")
			err := Errorf("yolo: %w", e1)

			got := StackTrace(err)
			attest.Subsequence(t, got, "ong/errors/errors_test.go:288")
		}
		{
			e1 := New("e1")
			e2 := New("e2")
			err := Join(e2, e1)

			got := StackTrace(err)
			attest.Subsequence(t, got, "ong/errors/errors_test.go:296")
		}
	})
}
