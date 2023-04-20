package middleware_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/komuw/ong/log"
	"github.com/komuw/ong/middleware"

	"golang.org/x/exp/slog"
)

func loginHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cspNonce := middleware.GetCspNonce(r.Context())
		_ = cspNonce // use CSP nonce

		_, _ = fmt.Fprint(w, "welcome to your favorite website.")
	}
}

func welcomeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		csrfToken := middleware.GetCsrfToken(r.Context())
		_ = csrfToken // use CSRF token

		_, _ = fmt.Fprint(w, "welcome.")
	}
}

func Example_getCspNonce() {
	l := log.New(os.Stdout, 100)(context.Background())
	handler := middleware.Get(
		loginHandler(),
		middleware.WithOpts("example.com", 443, "secretKey", middleware.DirectIpStrategy, l),
	)
	_ = handler // use handler

	// Output:
}

func Example_getCsrfToken() {
	l := log.New(os.Stdout, 100)(context.Background())
	handler := middleware.Get(
		welcomeHandler(),
		middleware.WithOpts("example.com", 443, "secretKey", middleware.DirectIpStrategy, l),
	)
	_ = handler // use handler

	// Output:
}

func ExampleGet() {
	l := log.New(os.Stdout, 100)(context.Background())
	opts := middleware.WithOpts("example.com", 443, "secretKey", middleware.DirectIpStrategy, l)
	handler := middleware.Get(loginHandler(), opts)
	_ = handler // use handler

	// Output:
}

func ExampleAll() {
	l := log.New(os.Stdout, 100)(context.Background())
	opts := middleware.WithOpts("example.com", 443, "secretKey", middleware.DirectIpStrategy, l)

	myHandler := func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "Hello from a HandleFunc \n")
	}

	handler := middleware.All(myHandler, opts)

	mux := http.NewServeMux()
	mux.HandleFunc("/", handler)

	// Output:
}

func ExampleWithOpts() {
	l := slog.New(slog.NewTextHandler(os.Stdout))
	opts := middleware.WithOpts(
		"example.com",
		443,
		"secretKey",
		// assuming your application is deployed behind cloudflare.
		middleware.SingleIpStrategy("CF-Connecting-IP"),
		l,
	)
	_ = opts

	// Output:
}
