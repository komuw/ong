package middleware_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/komuw/ong/config"
	"github.com/komuw/ong/log"
	"github.com/komuw/ong/middleware"
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
	l := log.New(context.Background(), os.Stdout, 100)
	handler := middleware.Get(
		loginHandler(),
		config.WithOpts("example.com", 443, "super-h@rd-Pa$1word", middleware.DirectIpStrategy, l),
	)
	_ = handler // use handler

	// Output:
}

func Example_getCsrfToken() {
	l := log.New(context.Background(), os.Stdout, 100)
	handler := middleware.Get(
		welcomeHandler(),
		config.WithOpts("example.com", 443, "super-h@rd-Pa$1word", middleware.DirectIpStrategy, l),
	)
	_ = handler // use handler

	// Output:
}

func ExampleGet() {
	l := log.New(context.Background(), os.Stdout, 100)
	opts := config.WithOpts("example.com", 443, "super-h@rd-Pa$1word", middleware.DirectIpStrategy, l)
	handler := middleware.Get(loginHandler(), opts)
	_ = handler // use handler

	// Output:
}

func ExampleAll() {
	l := log.New(context.Background(), os.Stdout, 100)
	opts := config.WithOpts("example.com", 443, "super-h@rd-Pa$1word", middleware.DirectIpStrategy, l)

	myHandler := http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "Hello from a HandleFunc \n")
		},
	)

	handler := middleware.All(myHandler, opts)

	mux := http.NewServeMux()
	mux.Handle("/", handler)

	// Output:
}

func ExampleWithOpts() {
	l := slog.New(slog.NewTextHandler(os.Stdout, nil))
	opts := config.WithOpts(
		"example.com",
		443,
		"super-h@rd-Pa$1word",
		// assuming your application is deployed behind cloudflare.
		middleware.SingleIpStrategy("CF-Connecting-IP"),
		l,
	)
	_ = opts

	// Output:
}
