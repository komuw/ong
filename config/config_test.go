package config

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/komuw/ong/internal/clientip"
	"github.com/komuw/ong/internal/tst"
	"github.com/komuw/ong/log"
	"go.akshayshah.org/attest"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	// call flag.Parse() here if TestMain uses flags
	goleak.VerifyTestMain(m)
}

func validOpts(t *testing.T) Opts {
	t.Helper()

	l := log.New(context.Background(), &bytes.Buffer{}, 500)
	return New(
		// The domain where our application will be available on.
		"example.com",
		// The https port that our application will be listening on.
		443,
		// The security key to use for securing signed data.
		"super-h@rd-Pas1word",
		// In this case, the actual client IP address is fetched from the given http header.
		SingleIpStrategy("CF-Connecting-IP"),
		// Logger.
		l,
		// log 90% of all responses that are either rate-limited or loadshed.
		90,
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
		func(r *http.Request) string { return r.Header.Get("Anti-Replay") },
		//
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
}

func TestNewMiddlewareOpts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		opt    func() middlewareOpts
		assert func(middlewareOpts)
	}{
		{
			name: "zero cache duration",
			opt: func() middlewareOpts {
				opt := validOpts(t)
				opt.middlewareOpts.CorsCacheDuration = 0 * time.Second
				return opt.middlewareOpts
			},
			assert: func(o middlewareOpts) { attest.Equal(t, o.CorsCacheDuration, 0) },
		},
		{
			name: "less than zero duration",
			opt: func() middlewareOpts {
				opt := validOpts(t)
				opt.middlewareOpts.CorsCacheDuration = 100 * time.Millisecond
				return opt.middlewareOpts
			},
			assert: func(o middlewareOpts) { attest.Equal(t, o.CorsCacheDuration, DefaultCorsCacheDuration) },
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opt := tt.opt()
			o := newMiddlewareOpts(
				opt.Domain,
				opt.HttpsPort,
				string(opt.SecretKey),
				opt.Strategy,
				opt.Logger,
				opt.RateShedSamplePercent,
				opt.RateLimit,
				opt.LoadShedSamplingPeriod,
				opt.LoadShedMinSampleSize,
				opt.LoadShedBreachLatency,
				opt.AllowedOrigins,
				opt.AllowedMethods,
				opt.AllowedHeaders,
				opt.AllowCredentials,
				opt.CorsCacheDuration,
				opt.CsrfTokenDuration,
				opt.SessionCookieDuration,
				opt.SessionAntiReplyFunc,
			)
			tt.assert(o)
		})
	}
}

func TestNewMiddlewareOptsPanics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		domain      string
		expectPanic bool
	}{
		// Some of them are taken from; https://github.com/golang/go/blob/go1.20.5/src/net/dnsname_test.go#L19-L34
		{
			name:        "good domain",
			domain:      "localhost",
			expectPanic: false,
		},
		{
			name:        "good domain B",
			domain:      "foo.com",
			expectPanic: false,
		},
		{
			name:        "good domain C",
			domain:      "bar.foo.com",
			expectPanic: false,
		},
		{
			name:        "bad domain",
			domain:      "a.b-.com",
			expectPanic: true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.expectPanic {
				attest.Panics(t, func() {
					newMiddlewareOpts(
						tt.domain,
						443,
						tst.SecretKey(),
						clientip.DirectIpStrategy,
						slog.Default(),
						DefaultRateShedSamplePercent,
						DefaultRateLimit,
						DefaultLoadShedSamplingPeriod,
						DefaultLoadShedMinSampleSize,
						DefaultLoadShedBreachLatency,
						nil,
						nil,
						nil,
						false,
						DefaultCorsCacheDuration,
						DefaultCsrfCookieDuration,
						DefaultSessionCookieDuration,
						DefaultSessionAntiReplyFunc,
					)
				})
			} else {
				newMiddlewareOpts(
					tt.domain,
					443,
					tst.SecretKey(),
					clientip.DirectIpStrategy,
					slog.Default(),
					DefaultRateShedSamplePercent,
					DefaultRateLimit,
					DefaultLoadShedSamplingPeriod,
					DefaultLoadShedMinSampleSize,
					DefaultLoadShedBreachLatency,
					nil,
					nil,
					nil,
					false,
					DefaultCorsCacheDuration,
					DefaultCsrfCookieDuration,
					DefaultSessionCookieDuration,
					DefaultSessionAntiReplyFunc,
				)
			}
		})
	}
}

func TestOpts(t *testing.T) {
	t.Parallel()

	t.Run("default DevOpts", func(t *testing.T) {
		t.Parallel()

		l := log.New(context.Background(), &bytes.Buffer{}, 500)
		got := DevOpts(l, tst.SecretKey())

		want := Opts{
			middlewareOpts: middlewareOpts{
				Domain:                 "localhost",
				HttpsPort:              65081,
				SecretKey:              secureKey(tst.SecretKey()),
				Strategy:               clientip.DirectIpStrategy,
				Logger:                 l,
				RateShedSamplePercent:  DefaultRateShedSamplePercent,
				RateLimit:              DefaultRateLimit,
				LoadShedSamplingPeriod: DefaultLoadShedSamplingPeriod,
				LoadShedMinSampleSize:  DefaultLoadShedMinSampleSize,
				LoadShedBreachLatency:  DefaultLoadShedBreachLatency,
				AllowedOrigins:         []string{},
				AllowedMethods:         []string{},
				AllowedHeaders:         []string{},
				CorsCacheDuration:      DefaultCorsCacheDuration,
				CsrfTokenDuration:      DefaultCsrfCookieDuration,
				SessionCookieDuration:  DefaultSessionCookieDuration,
				SessionAntiReplyFunc:   DefaultSessionAntiReplyFunc,
			},

			serverOpts: serverOpts{
				port:              65081,
				MaxBodyBytes:      DefaultMaxBodyBytes,
				ServerLogLevel:    DefaultServerLogLevel,
				ReadHeaderTimeout: 1 * time.Second,
				ReadTimeout:       2 * time.Second,
				WriteTimeout:      3 * time.Second,
				IdleTimeout:       113 * time.Second,
				DrainTimeout:      DefaultDrainDuration,
				Tls: tlsOpts{
					CertFile:              "/tmp/ong_dev_certificate.pem",
					KeyFile:               "/tmp/ong_dev_key.pem",
					AcmeEmail:             "",
					Hosts:                 []string{"localhost"},
					AcmeDirectoryUrl:      "",
					ClientCertificatePool: nil,
				},
				Host:          "127.0.0.1",
				ServerPort:    ":65081",
				ServerAddress: "127.0.0.1:65081",
				Network:       "tcp",
				HttpPort:      ":65080",
			},
		}

		attest.Equal(t, got, want)

		attest.Subsequence(t, got.SecretKey.String(), "REDACTED")
		attest.Subsequence(t, got.String(), "REDACTED")
		attest.Subsequence(t, got.GoString(), "REDACTED")
	})

	// t.Run("with opts", func(t *testing.T) {
	// 	t.Parallel()
	// })
}
