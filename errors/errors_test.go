package errors

import (
	"fmt"
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

func TestStackError(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		err := hello()

		sterr, ok := err.(*stackError)
		attest.True(t, ok)
		attest.Equal(t, sterr.Error(), "error in foo")

		stackTrace := sterr.getStackTrace()
		for _, v := range []string{
			"goweb/errors/errors_test.go:27",
			"goweb/errors/errors_test.go:20",
			"goweb/errors/errors_test.go:14",
			"goweb/errors/errors_test.go:32",
		} {
			attest.True(
				t,
				strings.Contains(stackTrace, v),
				attest.Sprintf("\n\t%s: not found in stackTrace: %s", v, stackTrace),
			)
		}
	})

	t.Run("formattting", func(t *testing.T) {
		err := hello()

		attest.Equal(t, fmt.Sprintf("%s", err), "error in foo")
		attest.Equal(t, fmt.Sprintf("%q", err), `"error in foo"`)
		attest.Equal(t, fmt.Sprintf("%v", err), "error in foo")

		extendedFormatting := fmt.Sprintf("%+v", err)
		for _, v := range []string{
			"goweb/errors/errors_test.go:27",
			"goweb/errors/errors_test.go:20",
			"goweb/errors/errors_test.go:14",
			"goweb/errors/errors_test.go:54",
		} {
			attest.True(
				t,
				strings.Contains(extendedFormatting, v),
				attest.Sprintf("\n\t%s: not found in extendedFormatting: %s", v, extendedFormatting),
			)
		}

	})

}
