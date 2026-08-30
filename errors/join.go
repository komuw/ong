package errors

import (
	"fmt"
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
// A non-nil error returned by Join implements Unwrap() []error.
// Its singular stack trace is the first stack found in the input errors.
// If no input error has a stack from this package, Join records its own stack.
func Join(errs ...error) error {
	// Contract:
	//   - Discard nil inputs and return nil when all inputs are nil.
	//   - Retain every non-nil input through Unwrap() []error.
	//   - Use the first available input stack, or capture the Join stack when none exists.
	n := 0
	for _, err := range errs {
		if err != nil {
			n++
		}
	}
	if n == 0 {
		return nil
	}

	joined := &joinError{errs: make([]error, 0, n)}
	for _, err := range errs {
		if err != nil {
			joined.errs = append(joined.errs, err)
		}
	}

	joined.stack = findStack(joined)
	if len(joined.stack) == 0 {
		joined.stack = captureStack(3)
	}

	return joined
}

type joinError struct {
	errs  []error
	stack []uintptr
}

func (e *joinError) Error() string {
	messages := make([]string, 0, len(e.errs))
	for _, err := range e.errs {
		messages = append(messages, err.Error())
	}
	return strings.Join(messages, "\n")
}

func (e *joinError) Unwrap() []error {
	return e.errs
}

func (e *joinError) stackTrace() []uintptr {
	return e.stack
}

func (e *joinError) Format(f fmt.State, verb rune) {
	formatError(f, verb, e.Error(), e.stack)
}
