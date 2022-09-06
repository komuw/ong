package xcontext

import (
	"context"
	"testing"
	"time"
)

type ctxKey string

var key = ctxKey("key")

func TestDetach(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	ctx = context.WithValue(ctx, key, "value")
	dctx := Detach(ctx)
	// Detached context has the same values.
	got, ok := dctx.Value(key).(string)
	if !ok || got != "value" {
		t.Errorf("Value: got (%v, %t), want 'value', true", got, ok)
	}
	// Detached context doesn't time out.
	time.Sleep(500 * time.Millisecond)
	if err := ctx.Err(); err != context.DeadlineExceeded {
		t.Fatalf("original context Err: got %v, want DeadlineExceeded", err)
	}
	if err := dctx.Err(); err != nil {
		t.Errorf("detached context Err: got %v, want nil", err)
	}
}

func ExampleDetach() {
	someFunc := func(ctx context.Context) {}
	someOtherFunc := func(ctx context.Context) {}

	foo := func() {
		ctx := context.WithValue(context.Background(), "my_key", "my_value")

		someFunc(ctx)

		// We need Detach here, because someOtherFunc can outlive the cancellation of the parent context.
		go someOtherFunc(Detach(ctx))
	}

	foo()
}
