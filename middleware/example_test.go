package middleware_test

import (
	"fmt"
	"io"
	"net/http"
	"os"

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

func Example_getCspNonce() {
	handler := middleware.Get(
		loginHandler(),
		middleware.WithOpts("example.com", 443, "secretKey", middleware.DirectIpStrategy, log.New(os.Stdout, 100)),
	)
	_ = handler // use handler

	// Output:
}

func welcomeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		csrfToken := middleware.GetCsrfToken(r.Context())
		_ = csrfToken // use CSRF token

		_, _ = fmt.Fprint(w, "welcome.")
	}
}

func Example_getCsrfToken() {
	handler := middleware.Get(
		welcomeHandler(),
		middleware.WithOpts("example.com", 443, "secretKey", middleware.DirectIpStrategy, log.New(os.Stdout, 100)),
	)
	_ = handler // use handler

	// Output:
}

func Example_get() {
	l := log.New(os.Stdout, 100)
	opts := middleware.WithOpts("example.com", 443, "secretKey", middleware.DirectIpStrategy, l)
	handler := middleware.Get(loginHandler(), opts)
	_ = handler // use handler

	// Output:
}

func Example_all() {
	l := log.New(os.Stdout, 100)
	opts := middleware.WithOpts("example.com", 443, "secretKey", middleware.DirectIpStrategy, l)

	myHandler := func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "Hello from a HandleFunc \n")
	}

	handler := middleware.All(myHandler, opts)

	http.HandleFunc("/", handler)
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		panic(err)
	}
}
