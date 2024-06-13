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

	// see: https://gitlab.com/tozd/go/errors/-/blob/v0.8.1/errors.go#L324
	switch u := err.(type) {
	default:
		// todo: handle this somehow
		return err
	case interface{ Unwrap() error }:
		ef := wrap(u.Unwrap(), 3)
		return &joinError{
			errs:       []error{err},
			stackError: ef.(*stackError), // ef is guaranteed to be a stackError since it comes from wrap()
		}
	}
}
