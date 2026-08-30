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

type stackProvider interface {
	stackTrace() []uintptr
}

func (e *stackError) stackTrace() []uintptr {
	return e.stack
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
	// Contract:
	//   - Return a non-nil error for every input string.
	//   - Capture a new origin stack.
	//   - Support %s, %q, %v, and %+v formatting.
	return wrap(stdErrors.New(text), 3)
}

// Wrap returns err with an origin stack trace.
// If err already contains a stack captured by this package, Wrap preserves that stack and the complete error chain.
func Wrap(err error) error {
	// Contract:
	//   - Return nil for a nil error.
	//   - Capture a stack only when the complete error tree has no package stack.
	//   - Preserve the innermost package stack and the complete error tree.
	return wrap(err, 3)
}

// Dwrap(aka deferred wrap) adds an origin stack trace to the error.
// It does nothing when *errp == nil and preserves a stack that was captured earlier.
func Dwrap(errp *error) {
	// Contract:
	//   - Do nothing when *errp is nil.
	//   - Apply the same stack and error-tree rules as Wrap.
	if *errp != nil {
		*errp = wrap(*errp, 3)
	}
}

// Errorf formats according to a format specifier and records an origin stack.
// It preserves the first stack found in errors wrapped with %w and keeps the standard unwrap relationships.
// It searches multiple wrapped errors from left to right.
func Errorf(format string, a ...any) error {
	// Contract:
	//   - Keep fmt.Errorf formatting and unwrap behavior.
	//   - Inherit a stack only through %w relationships.
	//   - Capture a new origin stack when no wrapped error has a package stack.
	return wrap(fmt.Errorf(format, a...), 3)
}

func wrap(err error, skip int) error {
	if err == nil {
		return nil
	}

	if _, ok := err.(stackProvider); ok {
		return err
	}

	if stack := findStack(err); len(stack) > 0 {
		return &stackError{
			err:   err,
			stack: stack,
		}
	}

	return &stackError{
		err:   err,
		stack: captureStack(skip + 1),
	}
}

func findStack(err error) []uintptr {
	if err == nil {
		return nil
	}

	switch u := err.(type) {
	case interface{ Unwrap() []error }:
		for _, child := range u.Unwrap() {
			if stack := findStack(child); len(stack) > 0 {
				return stack
			}
		}
	case interface{ Unwrap() error }:
		if stack := findStack(u.Unwrap()); len(stack) > 0 {
			return stack
		}
	}

	if e, ok := err.(stackProvider); ok {
		return e.stackTrace()
	}

	return nil
}

func captureStack(skip int) []uintptr {
	// limit stack size to 64 call depth.
	// `pkgsite/derrors` limits it to 16K(16 * 1024)
	// https://github.com/golang/pkgsite/blob/035bfc02f3faa0221e0edf90b0a21d3619c95fdd/internal/derrors/derrors.go#L261-L264
	stack := [64]uintptr{}
	// skip 0 identifies runtime.Callers, and skip 1 identifies captureStack.
	n := runtime.Callers(skip, stack[:])
	return stack[:n]
}

func formatStack(stack []uintptr) string {
	if len(stack) == 0 {
		return ""
	}

	var trace strings.Builder
	frames := runtime.CallersFrames(stack)
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

func (e *stackError) getStackTrace() string {
	return formatStack(e.stack)
}

// Format implements the fmt.Formatter interface.
func (e *stackError) Format(f fmt.State, verb rune) {
	formatError(f, verb, e.Error(), e.stack)
}

func formatError(f fmt.State, verb rune, message string, stack []uintptr) {
	switch verb {
	case 'v':
		if f.Flag('+') {
			trace := formatStack(stack)
			_, _ = io.WriteString(f, message)
			if !strings.Contains(message, trace) {
				_, _ = io.WriteString(f, trace)
			}
			return
		}
		fallthrough
	case 's':
		_, _ = io.WriteString(f, message)
	case 'q':
		_, _ = fmt.Fprintf(f, "%q", message)
	}
}

// StackTrace returns the innermost stack trace contained in err.
// It traverses single-cause and multi-cause errors, searching multi-cause errors from left to right.
func StackTrace(err error) string {
	// Contract:
	//   - Return an empty string when no package stack exists.
	//   - Search children before their wrappers.
	//   - Search multi-cause branches from left to right.
	return formatStack(findStack(err))
}
