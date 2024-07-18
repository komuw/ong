package errors

import (
	stdErrors "errors"
	"strings"
)

// Some of the code here is inspired(or taken from) by:
//   (a) https://github.com/golang/go/blob/go1.20.14/src/errors/join.go whose license(BSD 3-Clause) can be found here: https://github.com/golang/go/blob/go1.20.14/LICENSE

// Join returns an error that wraps the given errors.
// Any nil error values are discarded.
// Join returns nil if every value in errs is nil.
// The error formats as the concatenation of the strings obtained
// by calling the Error method of each element of errs, with a newline
// between each string.
//
// A non-nil error returned by Join implements the Unwrap() error method.
//
// It only returns the stack trace of the first error. Unwrap also only returns the first error.
//
// Note that this function is equivalent to the one in standard library mainly in spirit.
// This is not a direct replacement of the standard library one.
func Join(errs ...error) error {
	n := 0
	msgs := []string{}
	for _, err := range errs {
		if err != nil {
			n++
			msgs = append(msgs, err.Error())
		}
	}
	if n == 0 {
		return nil
	}

	if ef, ok := errs[0].(*stackError); ok {
		// If the first error was already a stack error, use its stacktrace.
		return &stackError{
			err:   stdErrors.New(strings.Join(msgs, "\n")),
			stack: ef.stack,
		}
	}

	return wrap(stdErrors.New(strings.Join(msgs, "\n")), 3)
}
