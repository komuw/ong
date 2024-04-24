package sync

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/sourcegraph/conc"
	"go.akshayshah.org/attest"
	"golang.org/x/sync/errgroup"
)

func TestSync(t *testing.T) {
	t.Parallel()

	t.Run("zero funcs", func(t *testing.T) {
		t.Parallel()

		{
			wgLimited, _ := New(context.Background(), 1)
			count := 0
			err := wgLimited.Go()
			attest.Ok(t, err)
			attest.Equal(t, count, 0)
		}

		{
			wgUnlimited, _ := New(context.Background(), -1)
			count := 0
			err := wgUnlimited.Go()
			attest.Ok(t, err)
			attest.Equal(t, count, 0)
		}
	})

	t.Run("one funcs", func(t *testing.T) {
		t.Parallel()

		{
			wgLimited, _ := New(context.Background(), 1)
			count := 0
			err := wgLimited.Go(func() error {
				count = count + 1
				return nil
			})
			attest.Ok(t, err)
			attest.Equal(t, count, 1)
		}

		{
			wgUnlimited, _ := New(context.Background(), -1)
			count := 0
			err := wgUnlimited.Go(func() error {
				count = count + 1
				return nil
			})
			attest.Ok(t, err)
			attest.Equal(t, count, 1)
		}
	})

	t.Run("multiple errors", func(t *testing.T) {
		t.Parallel()

		t.Run("limited", func(t *testing.T) {
			t.Parallel()

			wgLimited, _ := New(context.Background(), 1)
			err := wgLimited.Go(
				func() error {
					return fmt.Errorf("errorA number %d", 1)
				},
				func() error {
					return fmt.Errorf("errorA number %d", 2)
				},
				func() error {
					return fmt.Errorf("errorA number %d", 3)
				},
			)
			uw, ok := err.(interface{ Unwrap() []error })
			attest.True(t, ok)
			errs := uw.Unwrap()
			attest.Equal(t, len(errs), 3)
		})

		t.Run("unlimited", func(t *testing.T) {
			t.Parallel()

			wgUnlimited, _ := New(context.Background(), -1)
			err := wgUnlimited.Go(
				func() error {
					return fmt.Errorf("errorB number %d", 1)
				},
				func() error {
					return fmt.Errorf("errorB number %d", 2)
				},
				func() error {
					return fmt.Errorf("errorB number %d", 3)
				},
			)
			uw, ok := err.(interface{ Unwrap() []error })
			attest.True(t, ok)
			errs := uw.Unwrap()
			attest.Equal(t, len(errs), 3)
		})

		t.Run("unlimited concurrent", func(t *testing.T) {
			t.Parallel()

			wgUnlimited, _ := New(context.Background(), -1)
			ch := make(chan int, 2)

			{ // 1. first call
				go func() {
					_ = wgUnlimited.Go(
						func() error {
							return fmt.Errorf("errorC number %d", 1)
						},
						func() error {
							return fmt.Errorf("errorC number %d", 2)
						},
						func() error {
							return fmt.Errorf("errorC number %d", 3)
						},
					)
					ch <- 1
				}()
			}

			{ // 2. second call
				go func() {
					_ = wgUnlimited.Go(
						func() error {
							return fmt.Errorf("errorD number %d", 4)
						},
						func() error {
							return fmt.Errorf("errorD number %d", 5)
						},
						func() error {
							return fmt.Errorf("errorD number %d", 6)
						},
						func() error {
							return fmt.Errorf("errorD number %d", 7)
						},
						func() error {
							return fmt.Errorf("errorD number %d", 8)
						},
					)
					ch <- 1
				}()
			}

			{ // 3. wait
				a := <-ch
				b := <-ch
				attest.Equal(t, a, 1)
				attest.Equal(t, b, 1)

				err := wgUnlimited.Go(func() error {
					return fmt.Errorf("errorE number %d", 9)
				})
				uw, ok := err.(interface{ Unwrap() []error })
				attest.True(t, ok)
				errs := uw.Unwrap()
				attest.Equal(t, len(errs), 9)
			}
		})
	})

	t.Run("concurrency", func(t *testing.T) {
		t.Parallel()

		{
			wgLimited, _ := New(context.Background(), 1)

			funcs := []func() error{}
			for i := 0; i <= 4; i++ {
				funcs = append(funcs,
					func() error {
						return nil
					},
				)
			}

			go func() {
				err := wgLimited.Go(funcs...)
				attest.Ok(t, err)
			}()
			err := wgLimited.Go(funcs...)
			attest.Ok(t, err)
		}

		{
			wgUnlimited, _ := New(context.Background(), -1)

			funcs := []func() error{}
			for i := 0; i <= 4; i++ {
				funcs = append(funcs,
					func() error {
						return nil
					},
				)
			}

			go func() {
				err := wgUnlimited.Go(funcs...)
				attest.Ok(t, err)
			}()
			err := wgUnlimited.Go(funcs...)
			attest.Ok(t, err)
		}
	})
}

func BenchmarkSync(b *testing.B) {
	b.Logf("BenchmarkSync")

	b.Run("sync limited", func(b *testing.B) {
		count := 0
		wgLimited, _ := New(context.Background(), 1)
		funcs := []func() error{func() error {
			count = count + 1
			return nil
		}}

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			_ = wgLimited.Go(funcs...)
		}
	})

	b.Run("sync unlimited", func(b *testing.B) {
		count := 0
		wgUnlimited, _ := New(context.Background(), -1)
		funcs := []func() error{func() error {
			count = count + 1
			return nil
		}}

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			_ = wgUnlimited.Go(funcs...)
		}
	})

	b.Run("stdlib Waitgroup", func(b *testing.B) {
		count := 0
		stdWg := &sync.WaitGroup{}
		b.ReportAllocs()
		b.ResetTimer()

		for range b.N {
			stdWg.Add(1)
			go func() {
				count = count + 1
				stdWg.Done()
			}()
			stdWg.Wait()
		}
	})

	b.Run("errgroup Group limited", func(b *testing.B) {
		count := 0
		eWg, _ := errgroup.WithContext(context.Background())
		eWg.SetLimit(1)
		b.ReportAllocs()
		b.ResetTimer()

		for range b.N {
			eWg.Go(
				func() error {
					count = count + 1
					return nil
				},
			)
			_ = eWg.Wait()
		}
	})

	b.Run("errgroup Group unlimited", func(b *testing.B) {
		count := 0
		eWg, _ := errgroup.WithContext(context.Background())
		b.ReportAllocs()
		b.ResetTimer()

		for range b.N {
			eWg.Go(
				func() error {
					count = count + 1
					return nil
				},
			)
			_ = eWg.Wait()
		}
	})

	b.Run("conc WaitGroup", func(b *testing.B) {
		count := 0
		cWg := conc.NewWaitGroup()
		b.ReportAllocs()
		b.ResetTimer()

		for range b.N {
			cWg.Go(
				func() {
					count = count + 1
				},
			)
			cWg.Wait()
		}
	})
}

func panicTestHelper(t *testing.T, runFunc func() error, limit int) (recov interface{}) {
	t.Helper()

	defer func() {
		recov = recover()
	}()

	wgUnlimited, _ := New(context.Background(), limit)
	err := wgUnlimited.Go(runFunc)
	attest.Ok(t, err)

	return recov
}

// TestPanic is borrowed/inspired from: https://go-review.googlesource.com/c/sync/+/416555/2/errgroup/errgroup_test.go
func TestPanic(t *testing.T) {
	t.Parallel()

	/*
	   We have disabled this test subtest because `nilness` fails
	     nilness -test=true ./...
	       sync/sync_test.go:366:11: panic with nil value
	*/

	// t.Run("with nil", func(t *testing.T) {
	//  t.Parallel()
	//
	// 	// unlimited(-1), limited(1)
	// 	for limit := range []int{-1, 1} {
	// 		got := panicTestHelper(
	// 			t,
	// 			func() error {
	// 				panic(nil)
	// 			},
	// 			limit,
	// 		)
	// 		val, ok := got.(PanicError)
	// 		attest.True(t, ok)
	// 		gotStr := val.Error()
	// 		attest.Subsequence(t, gotStr, "nil")              // The panic message
	// 		attest.Subsequence(t, gotStr, "sync_test.go:350") // The place where the panic happened
	// 	}
	// })

	t.Run("some value", func(t *testing.T) {
		t.Parallel()

		// unlimited(-1), limited(1)
		for limit := range []int{-1, 1} {
			got := panicTestHelper(
				t,
				func() error {
					panic("hey hey")
				},
				limit,
			)
			_, ok := got.(PanicValue)
			attest.True(t, ok)
			gotStr := fmt.Sprintf("%+#s", got)
			attest.Subsequence(t, gotStr, "hey hey")          // The panic message
			attest.Subsequence(t, gotStr, "sync_test.go:370") // The place where the panic happened
		}
	})

	t.Run("some error", func(t *testing.T) {
		t.Parallel()

		// unlimited(-1), limited(1)
		for limit := range []int{-1, 1} {
			errPanic := errors.New("errPanic")

			got := panicTestHelper(
				t,
				func() error {
					panic(errPanic)
				},
				limit,
			)
			val, ok := got.(PanicError)
			attest.True(t, ok)
			gotStr := val.Error()
			attest.Subsequence(t, gotStr, errPanic.Error())   // The panic message
			attest.Subsequence(t, gotStr, "sync_test.go:392") // The place where the panic happened
		}
	})
}
