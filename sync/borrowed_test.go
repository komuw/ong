package sync

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// TestCancelCause is borrowed from: https://github.com/golang/sync/blob/v0.3.0/errgroup/go120_test.go

func TestCancelCause(t *testing.T) {
	t.Parallel()

	errDoom := errors.New("group_test: doomed")

	cases := []struct {
		name string
		errs []error
		want error
	}{
		{name: "no errors", want: nil},
		{name: "nil err", errs: []error{nil}, want: nil},
		{name: "one error", errs: []error{errDoom}, want: errDoom},
		{name: "two errors", errs: []error{errDoom, nil}, want: errDoom},
	}

	for _, tt := range cases {
		tc := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			funcs := []func() error{}
			for _, err := range tc.errs {
				err := err
				funcs = append(
					funcs,
					func() error { return err },
				)
			}

			errG := Go(context.Background(), 0, funcs...)
			if !errors.Is(errG, tc.want) {
				t.Errorf("got: %v. want: %v", errG, tc.want)
			}
		})
	}
}

// TestZeroGroup is borrowed from: https://github.com/golang/sync/blob/v0.3.0/errgroup/errgroup_test.go

func TestZeroGroup(t *testing.T) {
	// A zero value group is not valid.
}

// TestWithContext is borrowed from: https://github.com/golang/sync/blob/v0.3.0/errgroup/errgroup_test.go

func TestWithContext(t *testing.T) {
	errDoom := errors.New("group_test: doomed")

	cases := []struct {
		errs []error
		want error
	}{
		{want: nil},
		{errs: []error{nil}, want: nil},
		{errs: []error{errDoom}, want: errDoom},
		{errs: []error{errDoom, nil}, want: errDoom},
	}

	for _, tc := range cases {
		funcs := []func() error{}
		for _, err := range tc.errs {
			err := err
			funcs = append(
				funcs,
				func() error { return err },
			)
		}

		err := Go(context.Background(), 0, funcs...)
		if !errors.Is(err, tc.want) {
			t.Errorf("got: %v. want: %v", err, tc.want)
		}
	}
}

// TestGoLimit is borrowed from: https://github.com/golang/sync/blob/v0.3.0/errgroup/errgroup_test.go

func TestGoLimit(t *testing.T) {
	const limit = 10

	var active int32
	funcs := []func() error{}
	for i := 0; i <= 1<<10; i++ {
		funcs = append(
			funcs,
			func() error {
				n := atomic.AddInt32(&active, 1)
				if n > limit {
					return fmt.Errorf("saw %d active goroutines; want ≤ %d", n, limit)
				}
				time.Sleep(1 * time.Microsecond) // Give other goroutines a chance to increment active.
				atomic.AddInt32(&active, -1)
				return nil
			},
		)
	}

	if err := Go(context.Background(), limit, funcs...); err != nil {
		t.Fatal(err)
	}
}

// BenchmarkGo is borrowed from: https://github.com/golang/sync/blob/v0.3.0/errgroup/errgroup_test.go

func BenchmarkGo(b *testing.B) {
	fn := func() {}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := Go(
			context.Background(),
			0,
			func() error { fn(); return nil },
		); err != nil {
			b.Fatal(err)
		}
	}
}
