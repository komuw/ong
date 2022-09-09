package xcontext

import (
	"context"
	"testing"
	"time"
)

type ctxKey string

func TestDetach(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	key := ctxKey("key")
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

func log(ctx context.Context, logMsg string)   {}
func sendMail(ctx context.Context, msg string) {}

func ExampleDetach() {
	key := ctxKey("key")
	ctx := context.WithValue(context.Background(), key, "my_value")

	// Detach is not required here.
	log(ctx, "api called.")

	// We need to use Detach here, because sendMail(having been called in a goroutine) can outlive the cancellation of the parent context.
	go sendMail(Detach(ctx), "hello")

	// Output:
}
