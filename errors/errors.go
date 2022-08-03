// Package errors implements functions to manipulate errors.
package errors

import (
	"fmt"
	"io"
	"runtime"
	"strings"
)

// Most of the code here is insipired(or taken from) by:
//   (a) https://github.com/golang/pkgsite whose license(BSD 3-Clause "New") can be found here: https://github.com/golang/pkgsite/blob/24f94ffc546bde6aae0552efa6a940041d9d28e1/LICENSE
//   (b) https://www.komu.engineer/blogs/08/golang-stacktrace

// stackError is an error that contains a stack trace.
type stackError struct {
	stack [4]uintptr
	text  string
	err   error
}

func (e *stackError) Error() string {
	return e.text // ignore the stack
}

func (e *stackError) Unwrap() error {
	return e.err
}

// New returns an error with the supplied message. New also records the stack trace at the point it was called.
func New(text string) *stackError {
	return wrap(text, nil, 3)
}

// Wrap returns err, capturing a stack trace.
func Wrap(err error) *stackError {
	return wrap(err.Error(), err, 3)
}

func wrap(text string, err error, skip int) *stackError {
	stack := [4]uintptr{}
	// skip 0 identifies the frame for `runtime.Callers` itself and
	// skip 1 identifies the caller of `runtime.Callers`(ie of `wrap`).
	_ = runtime.Callers(skip, stack[:])

	return &stackError{
		stack: stack,
		text:  text,
		err:   err,
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

// StackTrace returns the stack trace contained in err, if it is a stackError, else an empty string.
func StackTrace(err error) string {
	sterr, ok := err.(*stackError)
	if !ok {
		return ""
	}
	return sterr.getStackTrace()
}
