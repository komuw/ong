package xcontext_test

import (
	"context"

	"github.com/komuw/ong/xcontext"
)

type ctxKey string

func log(ctx context.Context, logMsg string)   {}
func sendMail(ctx context.Context, msg string) {}

func ExampleDetach() {
	key := ctxKey("key")
	ctx := context.WithValue(context.Background(), key, "my_value")

	// Detach is not required here.
	log(ctx, "api called.")

	// We need to use Detach here, because sendMail(having been called in a goroutine) can outlive the cancellation of the parent context.
	go sendMail(xcontext.Detach(ctx), "hello")

	// Output:
}
