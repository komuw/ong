package sync

import (
	"context"
	"sync"
	"testing"

	"github.com/sourcegraph/conc"
	"go.akshayshah.org/attest"
	"golang.org/x/sync/errgroup"
)

func TestSync(t *testing.T) {
	t.Parallel()

	t.Run("zero value WaitGroup is valid", func(t *testing.T) {
		t.Parallel()

		{
			wg := WaitGroup{}
			count := 0
			err := wg.Go(func() error {
				count = count + 3
				return nil
			})
			attest.Ok(t, err)
			attest.Equal(t, count, 3)
		}

		{
			wg := &WaitGroup{}
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
		for n := 0; n < b.N; n++ {
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
		for n := 0; n < b.N; n++ {
			_ = wgUnlimited.Go(funcs...)
		}
	})

	b.Run("stdlib Waitgroup", func(b *testing.B) {
		count := 0
		stdWg := &sync.WaitGroup{}
		b.ReportAllocs()
		b.ResetTimer()

		for n := 0; n < b.N; n++ {
			stdWg.Add(1)
			go func() {
				count = count + 1
				stdWg.Done()
			}()
			stdWg.Wait()
		}
	})

	b.Run("errgroup WaitGroup limited", func(b *testing.B) {
		count := 0
		eWg, _ := errgroup.WithContext(context.Background())
		eWg.SetLimit(1)
		b.ReportAllocs()
		b.ResetTimer()

		for n := 0; n < b.N; n++ {
			eWg.Go(
				func() error {
					count = count + 1
					return nil
				},
			)
			_ = eWg.Wait()
		}
	})

	b.Run("errgroup WaitGroup unlimited", func(b *testing.B) {
		count := 0
		eWg, _ := errgroup.WithContext(context.Background())
		b.ReportAllocs()
		b.ResetTimer()

		for n := 0; n < b.N; n++ {
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

		for n := 0; n < b.N; n++ {
			cWg.Go(
				func() {
					count = count + 1
				},
			)
			cWg.Wait()
		}
	})
}
