package errors

import (
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
			"goweb/errors/errors_test.go:26",
			"goweb/errors/errors_test.go:19",
			"goweb/errors/errors_test.go:13",
			"goweb/errors/errors_test.go:31",
		} {
			attest.True(
				t,
				strings.Contains(stackTrace, v),
				attest.Sprintf("\n\t%s: not found in stackTrace: %s", v, stackTrace),
			)
		}
	})
}
