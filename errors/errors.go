// Package errors implements functions to manipulate errors.
package errors

import (
	stdErrors "errors"
	"fmt"
	"io"
	"runtime"
	"strings"
)

// Some of the code here is inspired(or taken from) by:
//   (a) https://github.com/golang/pkgsite whose license(BSD 3-Clause "New") can be found here: https://github.com/golang/pkgsite/blob/24f94ffc546bde6aae0552efa6a940041d9d28e1/LICENSE
//   (b) https://gitlab.com/tozd/go/errors whose license(Apache 2.0) can be found here: https://gitlab.com/tozd/go/errors/-/blob/v0.8.1/LICENSE
//   (c) https://www.komu.engineer/blogs/08/golang-stacktrace

// stackError is an implementation of error that adds stack trace support and error wrapping.
type stackError struct {
	stack []uintptr
	err   error
}

func (e *stackError) Error() string {
	return e.err.Error() // ignore the stack
}

// Unwrap unpacks wrapped errors.
func (e *stackError) Unwrap() error {
	return e.err
}

// Is reports whether target is a stackError
func (e *stackError) Is(target error) bool {
	_, ok := target.(*stackError)
	return ok
}

// New returns an error with the supplied message.
// It also records the stack trace at the point it was called.
//
// Error values implement [fmt.Formatter] and can be formatted by the fmt package. The following verbs are supported:
//
//	%s   print the error.
//	%v   see %s
//	%+v  print the error and stacktrace.
func New(text string) error {
	return wrap(stdErrors.New(text), 3)
}

// Wrap returns err, capturing a stack trace.
// It is a no-op if err had already been wrapped by this library.
func Wrap(err error) error {
	return wrap(err, 3)
}

// Dwrap(aka deferred wrap) adds stack traces to the error.
// It does nothing when *errp == nil.
func Dwrap(errp *error) {
	if *errp != nil {
		*errp = wrap(*errp, 3)
	}
}

func wrap(err error, skip int) *stackError {
	if err == nil {
		return nil
	}

	if Is(err, &stackError{}) {
		if e, ok := err.(*stackError); ok {
			return e
		}

		if e, ok := Unwrap(err).(*stackError); ok {
			return e
		}
	}

	// limit stack size to 64 call depth.
	// `pkgsite/derrors` limits it to 16K(16 * 1024)
	// https://github.com/golang/pkgsite/blob/035bfc02f3faa0221e0edf90b0a21d3619c95fdd/internal/derrors/derrors.go#L261-L264
	stack := [64]uintptr{}
	// skip 0 identifies the frame for `runtime.Callers` itself and
	// skip 1 identifies the caller of `runtime.Callers`(ie of `wrap`).
	n := runtime.Callers(skip, stack[:])

	return &stackError{
		err:   err,
		stack: stack[:n],
	}
}

func (e *stackError) getStackTrace() string {
	var trace strings.Builder
	frames := runtime.CallersFrames(e.stack[:])
	for {
		frame, more := frames.Next()
		if !strings.Contains(frame.File, "runtime/") { // we cant use something like "go/src/runtime/" since it will break for programs built using `go build -trimpath`
			trace.WriteString(fmt.Sprintf("\n%s:%d", frame.File, frame.Line))
		}
		if !more {
			break
		}
	}
	return trace.String()
}

// Format implements the fmt.Formatter interface
func (e *stackError) Format(f fmt.State, verb rune) {
	switch verb {
	case 'v':
		if f.Flag('+') {
			_, _ = io.WriteString(f, e.Error())
			_, _ = io.WriteString(f, e.getStackTrace())
			return
		}
		fallthrough
	case 's':
		_, _ = io.WriteString(f, e.Error())
	case 'q':
		_, _ = fmt.Fprintf(f, "%q", e.Error())
	}
}

// StackTrace returns the stack trace contained in err, if any, else an empty string.
func StackTrace(err error) string {
	if err == nil {
		return ""
	}

	if sterr, ok := err.(*stackError); ok {
		return sterr.getStackTrace()
	}

	if Is(err, &stackError{}) {
		switch u := err.(type) {
		default:
			// This is already handled by the `err.(*stackError)` case above.
			// todo: handle this somehow
			return ""
		case interface{ Unwrap() error }:
			ef := u.Unwrap()
			if sterr, ok := ef.(*stackError); ok {
				return sterr.getStackTrace()
			}
		}
	}

	return ""
}
