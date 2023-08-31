package config

import (
	"log/slog"
	"testing"

	"github.com/komuw/ong/internal/clientip"
	"github.com/komuw/ong/internal/tst"
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
