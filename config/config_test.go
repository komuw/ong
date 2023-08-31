package config

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"testing"

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
		fmt.Printf("%v", got)
		want := Opts{

			// port:              65081,
			// maxBodyBytes:      defaultMaxBodyBytes,
			// host:              "127.0.0.1",
			// network:           "tcp",
			// readHeaderTimeout: 1 * time.Second,
			// readTimeout:       2 * time.Second,
			// writeTimeout:      3 * time.Second,
			// handlerTimeout:    13 * time.Second,
			// idleTimeout:       113 * time.Second,
			// drainTimeout:      defaultDrainDuration,
			// serverPort:        ":65081",
			// serverAddress:     "127.0.0.1:65081",
			// httpPort:          ":65080",
			// pprofPort:         "65079",
			// tls: tlsOpts{
			// 	certFile:         "/tmp/ong_dev_certificate.pem",
			// 	keyFile:          "/tmp/ong_dev_key.pem",
			// 	domain:           "localhost",
			// 	acmeDirectoryUrl: "",
			// },
		}
		attest.Equal(
			t,
			got,
			want)
	})

	// t.Run("with opts", func(t *testing.T) {
	// 	t.Parallel()

	// 	certFile, keyFile := certKeyPaths()
	// 	got := withOpts(80, certFile, keyFile, "", "*.example.com", "")

	// 	want := Opts{
	// 		port:              80,
	// 		maxBodyBytes:      defaultMaxBodyBytes,
	// 		host:              "0.0.0.0",
	// 		network:           "tcp",
	// 		readHeaderTimeout: 1 * time.Second,
	// 		readTimeout:       2 * time.Second,
	// 		writeTimeout:      3 * time.Second,
	// 		handlerTimeout:    13 * time.Second,
	// 		idleTimeout:       113 * time.Second,
	// 		drainTimeout:      defaultDrainDuration,
	// 		serverPort:        ":80",
	// 		serverAddress:     "0.0.0.0:80",
	// 		httpPort:          ":79",
	// 		pprofPort:         "78",
	// 		tls: tlsOpts{
	// 			certFile:         "/tmp/ong_dev_certificate.pem",
	// 			keyFile:          "/tmp/ong_dev_key.pem",
	// 			domain:           "*.example.com",
	// 			acmeDirectoryUrl: "",
	// 		},
	// 	}
	// 	attest.Equal(t, got, want)
	// })

	// t.Run("default tls opts", func(t *testing.T) {
	// 	t.Parallel()

	// 	l := log.New(context.Background(), &bytes.Buffer{}, 500)
	// 	got := DevOpts(l)
	// 	want := Opts{
	// 		port:              65081,
	// 		maxBodyBytes:      defaultMaxBodyBytes,
	// 		host:              "127.0.0.1",
	// 		network:           "tcp",
	// 		readHeaderTimeout: 1 * time.Second,
	// 		readTimeout:       2 * time.Second,
	// 		writeTimeout:      3 * time.Second,
	// 		handlerTimeout:    13 * time.Second,
	// 		idleTimeout:       113 * time.Second,
	// 		drainTimeout:      defaultDrainDuration,
	// 		tls: tlsOpts{
	// 			certFile:         "/tmp/ong_dev_certificate.pem",
	// 			keyFile:          "/tmp/ong_dev_key.pem",
	// 			domain:           "localhost",
	// 			acmeDirectoryUrl: "",
	// 		},
	// 		serverPort:    ":65081",
	// 		serverAddress: "127.0.0.1:65081",
	// 		httpPort:      ":65080",
	// 		pprofPort:     "65079",
	// 	}
	// 	attest.Equal(t, got, want)
	// })
}
