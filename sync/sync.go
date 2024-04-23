// Package sync provides synchronization, error propagation, and Context
// cancelation for groups of goroutines working on subtasks of a common task.
package sync

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Some of the code here is inspired by(or taken from):
//   (a) https://github.com/golang/sync/tree/v0.3.0/errgroup whose license(BSD 3-Clause) can be found here: https://github.com/golang/sync/blob/v0.3.0/LICENSE
//   (b) https://go-review.googlesource.com/c/sync/+/416555
//   (c) https://github.com/sourcegraph/conc whose license(MIT) can be found here:                          https://github.com/sourcegraph/conc/blob/v0.3.0/LICENSE
//

// Group is a datastructure that waits for a collection of goroutines to finish.
// See [sync.WaitGroup]
//
// Use [New] to get a valid Group
type Group struct {
	mu sync.Mutex // protects wg when Group.Go is called concurrently.
	wg sync.WaitGroup

	cancel context.CancelCauseFunc

	// For limiting work.
	// See: golang.org/x/sync/errgroup
	sem chan struct{}

	errMu         sync.Mutex // protects err
	err           error
	collectedErrs []error

	//// TODO:
	panic interface{} // PanicError or PanicValue
}

// TODO: update docs.

// New returns a valid Group and a context(derived from ctx).
// The Group limits the number of active goroutines to at most n.
//
// The derived Context is canceled the first time Go returns.
// Unlike [golang.org/x/sync/errgroup.Group] it is not cancelled the first time a function passed to Go returns an error.
//
// n limits the number of active goroutines in this Group.
// If n is <=0, it indicates no limit.
func New(ctx context.Context, n int) (*Group, context.Context) {
	ctx, cancel := context.WithCancelCause(ctx)

	wg := &Group{cancel: cancel}
	if n > 0 {
		wg.sem = make(chan struct{}, n)
	}
	return wg, ctx
}

// Go calls each of the given functions in a new goroutine.
// It blocks until the new goroutine can be added without the number of
// active goroutines in the Group exceeding the configured limit.
//
// It also blocks until all function calls from the Go method have returned, then returns the concated non-nil errors(if any) from them.
// Unlike [golang.org/x/sync/errgroup.Group] the first call to return an error does not cancel the Group's context.
//
// If called concurrently, it will block until the previous call returns.
func (w *Group) Go(funcs ...func() error) error {
	countFuncs := len(funcs)
	if countFuncs <= 0 {
		if w.cancel != nil {
			w.cancel(w.err)
		}
		if w.panic != nil {
			panic(w.panic)
		}
		// if w.goexit { // TODO: add this.
		// 	runtime.Goexit()
		// }

		return nil
	}

	{
		w.mu.Lock()
		defer w.mu.Unlock()
	}

	{ // 1. User didn't set a limit when creating a [Group]
		if w.sem == nil {
			fmt.Println("\n\t ========= here ====") // TODO:

			w.wg.Add(countFuncs)
			for _, f := range funcs {
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
			w.err = errors.Join(w.collectedErrs...)
			if w.cancel != nil {
				w.cancel(w.err)
			}
			if w.panic != nil {
				panic(w.panic)
			}
			// if w.goexit { // TODO: add this.
			// 	runtime.Goexit()
			// }

			return w.err
		}
	}

	{ // 2. User did set a limit when creating a [Group]
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
		w.err = errors.Join(w.collectedErrs...)
		if w.cancel != nil {
			w.cancel(w.err)
		}
		if w.panic != nil {
			panic(w.panic)
		}
		// if w.goexit { // TODO: add this.
		// 	runtime.Goexit()
		// }
		return w.err
	}
}

func (w *Group) done() {
	fmt.Println("\n\t ========= here done ====") // TODO:

	{
		if v := recover(); v != nil {
			w.panic = addStack(v)
		}
	}

	if w.sem != nil {
		<-w.sem
	}
	w.wg.Done()
}
