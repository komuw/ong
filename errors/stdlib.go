package errors

import stdErrors "errors"

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
