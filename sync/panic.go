package sync

import (
	"bytes"
	"fmt"
	"runtime"
)

// A panicError wraps an error recovered from an unhandled panic
// when calling a function passed to Go.
type panicError struct {
	Recovered error
	Stack     []byte
}

func (p panicError) Error() string {
	// A Go Error method conventionally does not include a stack dump, but here we add it.
	if len(p.Stack) > 0 {
		return fmt.Sprintf("recovered from group: %v\n%s", p.Recovered, p.Stack)
	}
	return fmt.Sprintf("recovered from group: %v", p.Recovered)
}

func (p panicError) Unwrap() error { return p.Recovered }

// A panicValue wraps a value that does not implement the error interface
// recovered from an unhandled panic when calling a function passed to Go.
type panicValue struct {
	Recovered interface{}
	Stack     []byte
}

func (p panicValue) String() string {
	if len(p.Stack) > 0 {
		return fmt.Sprintf("recovered from group: %v\n%s", p.Recovered, p.Stack)
	}
	return fmt.Sprintf("recovered from group: %v", p.Recovered)
}

// addStack returns a PanicError or PanicValue that wraps v with a stack trace
// of the panicking goroutine.
func addStack(v interface{}) interface{} {
	// Taken from https://go-review.googlesource.com/c/sync/+/416555
	//
	stack := make([]byte, 2<<10)
	n := runtime.Stack(stack, false)
	for n == len(stack) {
		stack = make([]byte, len(stack)*2)
		n = runtime.Stack(stack, false)
	}
	stack = stack[:n]

	// The first line of the stack trace is of the form "goroutine N [status]:"
	// but by the time the panic reaches Wait the goroutine will no longer exist
	// and its status will have changed. Trim out the misleading line.
	if bytes.HasPrefix(stack, []byte("goroutine ")) {
		if line := bytes.IndexByte(stack, '\n'); line >= 0 {
			stack = stack[line+1:]
		}
	}

	if err, ok := v.(error); ok {
		return panicError{Recovered: err, Stack: stack}
	}
	return panicValue{Recovered: v, Stack: stack}
}
