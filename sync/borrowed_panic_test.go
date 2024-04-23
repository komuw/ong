package sync

// import (
// 	"context"
// 	"fmt"
// 	"testing"

// 	"go.akshayshah.org/attest"
// )

// // This are borrowed from: https://go-review.googlesource.com/c/sync/+/416555/2/errgroup/errgroup_test.go

// func terminateInGroup(t *testing.T, terminate func() error) (panicValue interface{}) {
// 	t.Helper()

// 	defer func() {
// 		panicValue = recover()
// 	}()

// 	g, ctx := New(context.Background(), -1) // TODO: add test for limited.

// 	waited := false
// 	g.Go(func() error {
// 		<-ctx.Done()
// 		waited = true
// 		return ctx.Err()
// 	})
// 	defer func() {
// 		if !waited {
// 			t.Errorf("did not wait for other goroutines to exit")
// 		}
// 	}()

// 	err := g.Go(terminate)
// 	t.Fatalf("g.Go() unexpectedly returned (with error %v)", err)
// 	return panicValue
// }

// // TODO: naming
// func helpMe(t *testing.T, runFunc func() error) (recov interface{}) {
// 	t.Helper()

// 	defer func() {
// 		recov = recover()
// 	}()

// 	wgUnlimited, _ := New(context.Background(), -1) // TODO: add test for limited.
// 	err := wgUnlimited.Go(runFunc)
// 	attest.Ok(t, err)

// 	return recov
// }

// func TestPanic(t *testing.T) {
// 	t.Run("with nil", func(t *testing.T) {
// 		got := helpMe(t,
// 			func() error {
// 				panic(nil)
// 			},
// 		)
// 		_, ok := got.(PanicError)
// 		attest.True(t, ok)
// 		gotStr := fmt.Sprintf("%+#s", got)
// 		attest.Subsequence(t, gotStr, "nil") // The panic message
// 	})

// 	t.Run("some value", func(t *testing.T) {
// 		got := helpMe(t,
// 			func() error {
// 				panic("hey hey")
// 			},
// 		)
// 		_, ok := got.(PanicValue)
// 		attest.True(t, ok)
// 		gotStr := fmt.Sprintf("%+#s", got)
// 		attest.Subsequence(t, gotStr, "hey hey")                   // The panic message
// 		attest.Subsequence(t, gotStr, "borrowed_panic_test.go:70") // The place where the panic happened
// 	})

// 	// t.Run("<nil>", func(t *testing.T) {
// 	// 	got := terminateInGroup(t, func() error {
// 	// 		panic(nil)
// 	// 	})
// 	// 	t.Logf("Wait panicked with: %v", got)
// 	// 	if gotV, ok := got.(PanicValue); !ok || gotV.Recovered != nil {
// 	// 		t.Errorf("want errgroup.PanicValue{Recovered: nil}")
// 	// 	}
// 	// })

// 	// t.Run("non-error", func(t *testing.T) {
// 	// 	const s = "some string"
// 	// 	got := terminateInGroup(t, func() error {
// 	// 		panic(s)
// 	// 	})
// 	// 	t.Logf("Wait panicked with: %v", got)
// 	// 	if gotV, ok := got.(errgroup.PanicValue); !ok || gotV.Recovered != s {
// 	// 		t.Errorf("want errgroup.PanicValue{Recovered: %q}", s)
// 	// 	}
// 	// })

// 	// t.Run("error", func(t *testing.T) {
// 	// 	var errPanic = errors.New("errPanic")
// 	// 	got := terminateInGroup(t, func() error {
// 	// 		panic(errPanic)
// 	// 	})
// 	// 	t.Logf("Wait panicked with: %v", got)
// 	// 	if err, ok := got.(error); !ok || !errors.Is(err, errPanic) {
// 	// 		t.Errorf("want errors.Is %v", errPanic)
// 	// 	}
// 	// })
// }
