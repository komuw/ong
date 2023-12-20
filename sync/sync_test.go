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
}
