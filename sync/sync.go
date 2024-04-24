// Package sync provides synchronization, error propagation, and Context
// cancelation for groups of goroutines working on subtasks of a common task.
package sync

import (
	"context"
	"errors"
	"runtime"
	"sync"
)

// Some of the code here is inspired by(or taken from):
//   (a) https://github.com/golang/sync/tree/v0.3.0/errgroup whose license(BSD 3-Clause) can be found here: https://github.com/golang/sync/blob/v0.3.0/LICENSE
//   (b) https://go-review.googlesource.com/c/sync/+/416555
//   (c) https://github.com/sourcegraph/conc whose license(MIT) can be found here:                          https://github.com/sourcegraph/conc/blob/v0.3.0/LICENSE
//   (d) https://github.com/kucherenkovova/safegroup whose license(BSD 3-Clause) can be found here:         https://github.com/kucherenkovova/safegroup/blob/v1.0.2/LICENSE
//

// group is a datastructure that waits for a collection of goroutines to finish.
// See [sync.WaitGroup]
//
// A zero value is not a valid group.
// Use [New] to get a valid group
type group struct {
	mu sync.Mutex // protects wg when group.Go is called concurrently.
	wg sync.WaitGroup

	// For limiting work.
	// See: golang.org/x/sync/errgroup
	sem chan struct{}

	errMu         sync.Mutex // protects err
	err           error
	collectedErrs []error
	panic         interface{} // PanicError or PanicValue
}

// New returns a group and a context(derived from ctx).
// A group waits for a collection of goroutines to finish. It has almost similar semantics to [sync.WaitGroup].
// It limits the number of active goroutines to at most n.
//
// The derived Context is canceled the first time Go returns.
// Unlike [golang.org/x/sync/errgroup.Group] it is not cancelled the first time a function passed to Go returns an error or panics.
//
// n limits the number of active goroutines in this group.
// If n is negative, the limit is set to [runtime.NumCPU]
func New(ctx context.Context, n int) (*group, context.Context) {
	wg := &group{}
	wg.sem = make(chan struct{}, runtime.NumCPU())
	if n > 0 {
		wg.sem = make(chan struct{}, n)
	}

	return wg, ctx // TODO: don't return ctx.
}

// TODO: docs
// TODO: since we create a new group everytime this func is called, this func cannot be called concurrently.
func Go(ctx context.Context, n int, funcs ...func() error) error {
	w := &group{}
	w.sem = make(chan struct{}, runtime.NumCPU())
	if n > 0 {
		w.sem = make(chan struct{}, n)
	}

	countFuncs := len(funcs)
	if countFuncs <= 0 {
		return nil
	}

	// { // TODO: This is not needed. Since group is not concurrent.
	// 	w.mu.Lock()
	// 	defer w.mu.Unlock()
	// }

	// Allow upto limit when creating a [group]
	count := 0
	for {
		if count == countFuncs {
			break
		}
		count = count + 1

		if count > countFuncs {
			panic("unreachable")
		}

		capacity := cap(w.sem)
		index := min(capacity, len(funcs))
		newFuncs := funcs[:index]
		funcs = funcs[index:]

		w.wg.Add(len(newFuncs))
		for _, f := range newFuncs {
			w.sem <- struct{}{}

			go func(f func() error) {
				defer w.done()
				if err := f(); err != nil {
					w.errMu.Lock()
					w.collectedErrs = append(w.collectedErrs, err)
					w.errMu.Unlock()
				}
			}(f)
		}
		w.wg.Wait()
	}
	if w.panic != nil { // TODO: should this be in a defer?
		panic(w.panic)
	}
	w.err = errors.Join(w.collectedErrs...)
	return w.err
}

// Go calls each of the given functions in a new goroutine.
// It blocks until the new goroutine can be added without the number of
// active goroutines in the group exceeding the configured limit.
//
// It also blocks until all function calls from the Go method have returned, then returns the concated non-nil errors(if any) from them.
// If any of those functions panic, Go will also propagate that panic.
// Unlike [golang.org/x/sync/errgroup.Group] the first call to return an error(or panics) does not cancel the group's context.
//
// If called concurrently, it will block until the previous call returns.
func (w *group) Go(funcs ...func() error) error {
	countFuncs := len(funcs)
	if countFuncs <= 0 {
		if w.panic != nil {
			panic(w.panic)
		}

		return nil
	}

	{
		w.mu.Lock()
		defer w.mu.Unlock()
	}

	// Allow upto limit when creating a [group]
	count := 0
	for {
		if count == countFuncs {
			break
		}
		count = count + 1

		if count > countFuncs {
			panic("unreachable")
		}

		capacity := cap(w.sem)
		index := min(capacity, len(funcs))
		newFuncs := funcs[:index]
		funcs = funcs[index:]

		w.wg.Add(len(newFuncs))
		for _, f := range newFuncs {
			w.sem <- struct{}{}

			go func(f func() error) {
				defer w.done()
				if err := f(); err != nil {
					w.errMu.Lock()
					w.collectedErrs = append(w.collectedErrs, err)
					w.errMu.Unlock()
				}
			}(f)
		}
		w.wg.Wait()
	}
	if w.panic != nil { // TODO: should this be in a defer?
		panic(w.panic)
	}
	w.err = errors.Join(w.collectedErrs...)
	return w.err
}

func (w *group) done() {
	if v := recover(); v != nil {
		w.panic = addStack(v)
	}

	<-w.sem
	w.wg.Done()
}
