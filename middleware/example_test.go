package middleware_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

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
		middleware.WithOpts("example.com", 443, "super-h@rd-Pa$1word", middleware.DirectIpStrategy, l),
	)
	_ = handler // use handler

	// Output:
}

func Example_getCsrfToken() {
	l := log.New(context.Background(), os.Stdout, 100)
	handler := middleware.Get(
		welcomeHandler(),
		middleware.WithOpts("example.com", 443, "super-h@rd-Pa$1word", middleware.DirectIpStrategy, l),
	)
	_ = handler // use handler

	// Output:
}

func ExampleNew() {
	l := log.New(context.Background(), os.Stdout, 100)
	opts := middleware.New(
		"example.com",
		443,
		"super-h@rd-Pa$1word",
		// In this case, the actual client IP address is fetched from the given http header.
		middleware.SingleIpStrategy("CF-Connecting-IP"),
		l,
		// Allow access from these origins for CORs.
		[]string{"*example.net", "example.org"},
		// Allow only GET and POST for CORs.
		[]string{"GET", "POST"},
		// Allow all http headers for CORs.
		[]string{"*"},
		// Cache CORs preflight requests for 1day.
		24*time.Hour,
		// Expire csrf cookie after 3days.
		3*24*time.Hour,
		// Sample response latencies over a 5 minute window to determine if to loadshed.
		5*time.Minute,
		// If the number of responses in the last 5minutes is less than 10, do not make a loadshed determination.
		10,
		// If the p99 response latencies, over the last 5minutes is more than 200ms, then start loadshedding.
		200*time.Millisecond,
		// If a particular IP address sends more than 13 requests per second, throttle requests from that IP.
		13.0,
		// Expire session cookie after 6hours.
		6*time.Hour,
	)
	handler := middleware.Get(loginHandler(), opts)
	_ = handler // use handler

	// Output:
}

func ExampleGet() {
	l := log.New(context.Background(), os.Stdout, 100)
	opts := middleware.WithOpts("example.com", 443, "super-h@rd-Pa$1word", middleware.DirectIpStrategy, l)
	handler := middleware.Get(loginHandler(), opts)
	_ = handler // use handler

	// Output:
}

func ExampleAll() {
	l := log.New(context.Background(), os.Stdout, 100)
	opts := middleware.WithOpts("example.com", 443, "super-h@rd-Pa$1word", middleware.DirectIpStrategy, l)

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
	opts := middleware.WithOpts(
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
