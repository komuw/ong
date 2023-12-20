package sync

import (
	"context"
	"testing"

	"github.com/sourcegraph/conc"
	"golang.org/x/sync/errgroup"

	"go.akshayshah.org/attest"
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

func TestErrGroup(t *testing.T) {
	t.Parallel()

	t.Run("concurrency", func(t *testing.T) {
		t.Parallel()

		{
			wgLimited, _ := errgroup.WithContext(context.Background())
			wgLimited.SetLimit(1)

			go func() {
				wgLimited.Go(func() error {
					return nil
				})
			}()
			wgLimited.Go(func() error {
				return nil
			})

			err := wgLimited.Wait()
			attest.Ok(t, err)
		}

		{
			wgUnlimited, _ := errgroup.WithContext(context.Background())

			go func() {
				wgUnlimited.Go(func() error {
					return nil
				})
			}()
			wgUnlimited.Go(func() error {
				return nil
			})

			err := wgUnlimited.Wait()
			attest.Ok(t, err)
		}
	})
}

func TestConc(t *testing.T) {
	t.Parallel()

	t.Run("concurrency", func(t *testing.T) {
		t.Parallel()

		{
			wgLimited := conc.NewWaitGroup()

			go func() {
				wgLimited.Go(func() {
					return
				})
				wgLimited.Wait()
			}()
			wgLimited.Go(func() {
				return
			})

			wgLimited.Wait()
		}
	})
}
