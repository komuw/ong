package config

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/komuw/ong/internal/clientip"
	"github.com/komuw/ong/internal/tst"
	"github.com/komuw/ong/log"
	"go.akshayshah.org/attest"
)

func TestNewMiddlewareOpts(t *testing.T) {
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
					NewMiddlewareOpts(
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
						DefaultCorsCacheDuration,
						DefaultCsrfCookieDuration,
						DefaultSessionCookieDuration,
					)
				})
			} else {
				NewMiddlewareOpts(
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
					DefaultCorsCacheDuration,
					DefaultCsrfCookieDuration,
					DefaultSessionCookieDuration,
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
				SecretKey:              tst.SecretKey(),
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
			},

			serverOpts: serverOpts{
				port:              65081,
				MaxBodyBytes:      defaultMaxBodyBytes,
				ServerLogLevel:    defaultServerLogLevel,
				ReadHeaderTimeout: 1 * time.Second,
				ReadTimeout:       2 * time.Second,
				WriteTimeout:      3 * time.Second,
				IdleTimeout:       113 * time.Second,
				DrainTimeout:      defaultDrainDuration,
				Tls: tlsOpts{
					CertFile:              "/tmp/ong_dev_certificate.pem",
					KeyFile:               "/tmp/ong_dev_key.pem",
					AcmeEmail:             "",
					Domain:                "localhost",
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
	})

	// t.Run("with opts", func(t *testing.T) {
	// 	t.Parallel()
	// })
}
