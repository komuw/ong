package sync

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/sourcegraph/conc"
	"go.akshayshah.org/attest"
	"golang.org/x/sync/errgroup"
)

func TestSync(t *testing.T) {
	t.Parallel()

	t.Run("zero value Group is valid", func(t *testing.T) {
		t.Parallel()

		{
			wg := Group{}
			count := 0
			err := wg.Go(func() error {
				count = count + 3
				return nil
			})
			attest.Ok(t, err)
			attest.Equal(t, count, 3)
		}

		{
			wg := &Group{}
			count := 0
			err := wg.Go(func() error {
				count = count + 7
				return nil
			})
			attest.Ok(t, err)
			attest.Equal(t, count, 7)
		}
	})

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

// TODO: naming
func helpMe(t *testing.T, runFunc func() error) (recov interface{}) {
	t.Helper()

	defer func() {
		recov = recover()
	}()

	wgUnlimited, _ := New(context.Background(), -1) // TODO: add test for limited.
	err := wgUnlimited.Go(runFunc)
	attest.Ok(t, err)

	return recov
}

func TestPanic(t *testing.T) {
	t.Run("with nil", func(t *testing.T) {
		got := helpMe(t,
			func() error {
				panic(nil)
			},
		)
		_, ok := got.(PanicError)
		attest.True(t, ok)
		gotStr := fmt.Sprintf("%+#s", got)
		attest.Subsequence(t, gotStr, "nil") // The panic message
	})

	t.Run("some value", func(t *testing.T) {
		got := helpMe(t,
			func() error {
				panic("hey hey")
			},
		)
		_, ok := got.(PanicValue)
		attest.True(t, ok)
		gotStr := fmt.Sprintf("%+#s", got)
		attest.Subsequence(t, gotStr, "hey hey")          // The panic message
		attest.Subsequence(t, gotStr, "sync_test.go:374") // The place where the panic happened
	})

	// t.Run("<nil>", func(t *testing.T) {
	// 	got := terminateInGroup(t, func() error {
	// 		panic(nil)
	// 	})
	// 	t.Logf("Wait panicked with: %v", got)
	// 	if gotV, ok := got.(PanicValue); !ok || gotV.Recovered != nil {
	// 		t.Errorf("want errgroup.PanicValue{Recovered: nil}")
	// 	}
	// })

	// t.Run("non-error", func(t *testing.T) {
	// 	const s = "some string"
	// 	got := terminateInGroup(t, func() error {
	// 		panic(s)
	// 	})
	// 	t.Logf("Wait panicked with: %v", got)
	// 	if gotV, ok := got.(errgroup.PanicValue); !ok || gotV.Recovered != s {
	// 		t.Errorf("want errgroup.PanicValue{Recovered: %q}", s)
	// 	}
	// })

	// t.Run("error", func(t *testing.T) {
	// 	var errPanic = errors.New("errPanic")
	// 	got := terminateInGroup(t, func() error {
	// 		panic(errPanic)
	// 	})
	// 	t.Logf("Wait panicked with: %v", got)
	// 	if err, ok := got.(error); !ok || !errors.Is(err, errPanic) {
	// 		t.Errorf("want errors.Is %v", errPanic)
	// 	}
	// })
}
