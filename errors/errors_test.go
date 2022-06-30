package errors

import (
	stdErrors "errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/akshayjshah/attest"
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
				"ong/errors/errors_test.go:61",
			} {
				attest.True(
					t,
					strings.Contains(stackTrace, v),
					attest.Sprintf("\n\t%s: not found in stackTrace: %s", v, stackTrace),
				)
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
				"ong/errors/errors_test.go:85",
			} {
				attest.True(
					t,
					strings.Contains(stackTrace, v),
					attest.Sprintf("\n\t%s: not found in stackTrace: %s", v, stackTrace),
				)
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
			"ong/errors/errors_test.go:109",
		} {
			attest.True(
				t,
				strings.Contains(extendedFormatting, v),
				attest.Sprintf("\n\t%s: not found in extendedFormatting: %s", v, extendedFormatting),
			)
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
	})
}
