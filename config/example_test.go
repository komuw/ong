package config_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/komuw/ong/config"
	"github.com/komuw/ong/log"
	"github.com/komuw/ong/middleware"
)

func loginHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "welcome to your favorite website.")
	}
}

func ExampleNew() {
	l := log.New(context.Background(), os.Stdout, 100)
	opts := config.New(
		// The domain where our application will be available on.
		"example.com",
		// The https port that our application will be listening on.
		443,
		// The security key to use for securing signed data.
		"super-h@rd-Pas1word",
		// In this case, the actual client IP address is fetched from the given http header.
		config.SingleIpStrategy("CF-Connecting-IP"),
		// function to log in middlewares.
		func(_ http.ResponseWriter, r http.Request, statusCode int, fields []any) {
			reqL := log.WithID(r.Context(), l)
			reqL.Info("request-and-response", fields...)
		},
		// If a particular IP address sends more than 13 requests per second, throttle requests from that IP.
		13.0,
		// Sample response latencies over a 5 minute window to determine if to loadshed.
		5*time.Minute,
		// If the number of responses in the last 5minutes is less than 10, do not make a loadshed determination.
		10,
		// If the p99 response latencies, over the last 5minutes is more than 200ms, then start loadshedding.
		200*time.Millisecond,
		// Allow access from these origins for CORs.
		[]string{"http://example.net", "https://example.org"},
		// Allow only GET and POST for CORs.
		[]string{"GET", "POST"},
		// Allow all http headers for CORs.
		[]string{"*"},
		// Do not allow requests to include user credentials like cookies, HTTP authentication or client side SSL certificates
		false,
		// Cache CORs preflight requests for 1day.
		24*time.Hour,
		// Expire csrf cookie after 3days.
		3*24*time.Hour,
		// Expire session cookie after 6hours.
		6*time.Hour,
		// Use a given header to try and mitigate against replay-attacks.
		func(r http.Request) string { return r.Header.Get("Anti-Replay") },
		//
		// Logger.
		l,
		// The maximum size in bytes for incoming request bodies.
		2*1024*1024,
		// Log level of the logger that will be passed into [http.Server.ErrorLog]
		slog.LevelError,
		// Read header, Read, Write, Idle timeouts respectively.
		1*time.Second,
		2*time.Second,
		4*time.Second,
		4*time.Minute,
		// The duration to wait for after receiving a shutdown signal and actually starting to shutdown the server.
		17*time.Second,
		// Tls certificate and key. This are set to empty string since we wont be using them.
		"",
		"",
		// Email address to use when procuring TLS certificates from an ACME authority.
		"my-acme@example.com",
		// The hosts that we will allow to fetch certificates for.
		[]string{"api.example.com", "example.com"},
		// The ACME certificate authority to use.
		"https://acme-staging-v02.api.letsencrypt.org/directory",
		// [x509.CertPool], that will be used to verify client certificates
		nil,
	)
	handler := middleware.Get(loginHandler(), opts)
	_ = handler // use handler

	// Output:
}
