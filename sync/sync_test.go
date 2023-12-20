package sync

import (
	"context"
	"testing"

	"go.akshayshah.org/attest"
)

func TestSync(t *testing.T) {
	t.Parallel()

	wgLimited, _ := New(context.Background(), 1)
	wgUnlimited, _ := New(context.Background(), -1)

	t.Run("zero funcs", func(t *testing.T) {
		t.Parallel()

		{
			count := 0
			err := wgLimited.Go()
			attest.Ok(t, err)
			attest.Equal(t, count, 0)
		}

		{
			count := 0
			err := wgUnlimited.Go()
			attest.Ok(t, err)
			attest.Equal(t, count, 0)
		}
	})

	t.Run("one funcs", func(t *testing.T) {
		t.Parallel()

		{
			count := 0
			err := wgLimited.Go(func() error {
				count = count + 1
				return nil
			})
			attest.Ok(t, err)
			attest.Equal(t, count, 1)
		}

		{
			count := 0
			err := wgUnlimited.Go(func() error {
				count = count + 1
				return nil
			})
			attest.Ok(t, err)
			attest.Equal(t, count, 1)
		}
	})

	// go func() {
	// 	{
	// 		err := wg.Go(func() error {
	// 			count = count + 1
	// 			fmt.Println("\n\t count1: ", count)
	// 			return nil
	// 		})

	// 		attest.Ok(t, err)
	// 	}

	// 	{
	// 		err := wg.Go(func() error {
	// 			count = count + 1
	// 			fmt.Println("\n\t count1: ", count)
	// 			return nil
	// 		})

	// 		attest.Ok(t, err)
	// 	}
	// }()

	// var active int32
	// funcs := []func() error{}
	// for i := 0; i <= 1<<10; i++ {
	// 	funcs = append(
	// 		funcs,
	// 		func() error {
	// 			n := atomic.AddInt32(&active, 1)
	// 			if n > limit {
	// 				return fmt.Errorf("saw %d active goroutines; want ≤ %d", n, limit)
	// 			}
	// 			time.Sleep(1 * time.Microsecond) // Give other goroutines a chance to increment active.
	// 			atomic.AddInt32(&active, -1)
	// 			return nil
	// 		},
	// 	)
	// }

	// if err := g.Go(funcs...); err != nil {
	// 	t.Fatal(err)
	// }
}
