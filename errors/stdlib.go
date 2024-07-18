package errors

import (
	stdErrors "errors"
	"fmt"
)

// As is a pass through to the same func from the standard library errors package.
func As(err error, target any) bool {
	return stdErrors.As(err, target)
}

// Is is a pass through to the same func from the standard library errors package.
func Is(err, target error) bool {
	return stdErrors.Is(err, target)
}

// Unwrap is a pass through to the same func from the standard library errors package.
func Unwrap(err error) error {
	return stdErrors.Unwrap(err)
}

// Errorf is equivalent to the one in standard library mainly in spirit.
func Errorf(format string, a ...any) error {
	err := fmt.Errorf(format, a...)

	var stack []uintptr
	for _, e := range a {
		if ef, ok := e.(*stackError); ok {
			stack = ef.stack
		}
	}

	if len(stack) > 0 {
		return &stackError{
			err:   err,
			stack: stack,
		}
	}

	return wrap(err, 3)
}
