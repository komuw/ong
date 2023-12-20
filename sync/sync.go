// Package sync provides synchronization, error propagation, and Context
// cancelation for groups of goroutines working on subtasks of a common task.
package sync

import (
	"context"
	"sync"
)

// Some of the code here is inspired by(or taken from):
//   (a) https://github.com/golang/sync/tree/v0.3.0/errgroup whose license(BSD 3-Clause) can be found here: https://github.com/golang/sync/blob/v0.3.0/LICENSE
//   (b) https://github.com/sourcegraph/conc whose license(MIT) can be found here:                          https://github.com/sourcegraph/conc/blob/v0.3.0/LICENSE
//

// WaitGroup is a datastructure that waits for a collection of goroutines to finish.
// See [sync.WaitGroup]
//
// Use [New] to get a valid Waitgroup
type WaitGroup struct {
	mu sync.Mutex // protects wg when WaitGroup.Go is called concurrently & goroutines have been limited.
	wg sync.WaitGroup

	cancel context.CancelCauseFunc

	// For limiting work.
	// See: golang.org/x/sync/errgroup
	sem chan struct{}

	errOnce sync.Once
	err     error
}

// New returns a valid WaitGroup and a context(derived from ctx).
// The WaitGroup limits the number of active goroutines to at most n.
//
// The derived Context is canceled the first time a function passed to Go
// returns an error or the first time Go returns, whichever occurs first.
//
// n limits the number of active goroutines in this WaitGroup.
// If n is <=0, it indicates no limit.
func New(ctx context.Context, n int) (*WaitGroup, context.Context) {
	ctx, cancel := context.WithCancelCause(ctx)

	wg := &WaitGroup{cancel: cancel}
	if n > 0 {
		wg.sem = make(chan struct{}, n)
	}
	return wg, ctx
}

// Go calls each of the given functions in a new goroutine.
// It blocks until the new goroutine can be added without the number of
// active goroutines in the WaitGroup exceeding the configured limit.
//
// It also blocks until all function calls from the Go method have returned, then returns the first non-nil error (if any) from them.
// The first call to return an error cancels the WaitGroup's context.
func (w *WaitGroup) Go(funcs ...func() error) error {
	countFuncs := len(funcs)
	if countFuncs <= 0 {
		if w.cancel != nil {
			w.cancel(w.err)
		}
		return nil
	}

	{ // 1. User didn't set a limit when creating a [WaitGroup]
		if w.sem == nil {
			w.wg.Add(countFuncs)
			for _, f := range funcs {
				go func(f func() error) {
					defer w.done()
					err := f()
					if err != nil {
						w.errOnce.Do(func() {
							w.err = err
							if w.cancel != nil {
								w.cancel(w.err)
							}
						})
					}
				}(f)
			}
			w.wg.Wait()
			if w.cancel != nil {
				w.cancel(w.err)
			}

			return w.err
		}
	}

	{ // 2. User did set a limit when creating a [WaitGroup]
		w.mu.Lock()
		defer w.mu.Unlock()

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
					err := f()
					if err != nil {
						w.errOnce.Do(func() {
							w.err = err
							if w.cancel != nil {
								w.cancel(w.err)
							}
						})
					}
				}(f)
			}

			w.wg.Wait()

		}

		if w.cancel != nil {
			w.cancel(w.err)
		}
		return w.err
	}
}

func (w *WaitGroup) done() {
	if w.sem != nil {
		<-w.sem
	}
	w.wg.Done()
}
