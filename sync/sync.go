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

// Go waits for a collection of goroutines to finish. It has almost similar semantics to [sync.WaitGroup] and/or [golang.org/x/sync/errgroup.Group]
// It limits the number of active goroutines to at most n. If n<=0, the limit is set to [runtime.NumCPU]
//
// It calls each of the given functions in a new goroutine and blocks until the new goroutine can be added without the number of
// active goroutines in the group exceeding the configured limit.
//
// It also blocks until all function calls have returned, then returns the concated non-nil errors(if any) from them.
// If any of those functions panic, [Go] will propagate that panic.
// Unlike [golang.org/x/sync/errgroup.Group] errors and panics do not cancel the context.
//
// If callers of [Go] cancel ctx, it will return after the current executing func has finished.
func Go(ctx context.Context, n int, funcs ...func() error) error {
	var (
		wg                        = &sync.WaitGroup{}
		panicKy       interface{} = nil // PanicError or PanicValue
		errRet        error
		errMu         sync.Mutex // protects collectedErrs
		collectedErrs []error
		sem           = make(chan struct{}, runtime.NumCPU())
	)
	if n > 0 {
		sem = make(chan struct{}, n)
	}

	countFuncs := len(funcs)
	if countFuncs <= 0 {
		return nil
	}

	// Allow upto limit when creating a [group]
	count := 0
	for {
		select {
		default:
			if count == countFuncs {
				if panicKy != nil {
					panic(panicKy)
				}
				errRet = errors.Join(collectedErrs...)
				return errRet
			}
			count = count + 1

			if count > countFuncs {
				panic("unreachable")
			}

			capacity := cap(sem)
			index := min(capacity, len(funcs))
			newFuncs := funcs[:index]
			funcs = funcs[index:]

			wg.Add(len(newFuncs))
			for _, f := range newFuncs {
				sem <- struct{}{}

				go func(f func() error) {
					{ // done
						defer func() {
							if v := recover(); v != nil {
								panicKy = addStack(v)
							}
							<-sem
							wg.Done()
						}()
					}

					if err := f(); err != nil {
						errMu.Lock()
						collectedErrs = append(collectedErrs, err)
						errMu.Unlock()
					}
				}(f)
			}
			wg.Wait()
		case <-ctx.Done():
			if panicKy != nil {
				panic(panicKy)
			}
			errRet = errors.Join(collectedErrs...)
			return errRet
		}
	}
}
