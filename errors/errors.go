// Package errors implements functions to manipulate errors.
package errors

import (
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
)

// Most of the code here is insipired(or taken from) by:
//   (a) https://github.com/golang/pkgsite whose license(BSD 3-Clause "New") can be found here: https://github.com/golang/pkgsite/blob/24f94ffc546bde6aae0552efa6a940041d9d28e1/LICENSE
//   (b) https://www.komu.engineer/blogs/08/golang-stacktrace

// stackError wraps an error and adds a stack trace.
type stackError struct {
	stack []uintptr
	err   error
}

func (e *stackError) Error() string {
	return e.err.Error() // ignore the stack
}

func (e *stackError) Unwrap() error {
	return e.err
}

func New(text string) *stackError {
	return Wrap(errors.New(text))
}

// Wrap returns err, capturing a stack trace.
func Wrap(err error) *stackError {
	stack := make([]uintptr, 50)
	length := runtime.Callers(3, stack[:])
	return &stackError{
		err:   err,
		stack: stack[:length],
	}
}

func (e *stackError) getStackTrace() string {
	var trace strings.Builder
	frames := runtime.CallersFrames(e.stack)
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
			io.WriteString(f, e.Error())
			io.WriteString(f, e.getStackTrace())
			return
		}
		fallthrough
	case 's':
		io.WriteString(f, e.Error())
	case 'q':
		fmt.Fprintf(f, "%q", e.Error())
	}
}

func StackTrace(err error) string {
	sterr, ok := err.(*stackError)
	if !ok {
		return ""
	}
	return sterr.getStackTrace()
}
