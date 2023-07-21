package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"testing"

	"github.com/komuw/ong/log"
	"go.akshayshah.org/attest"
)

func TestGetTlsConfig(t *testing.T) {
	t.Parallel()

	l := log.New(&bytes.Buffer{}, 500)(context.Background())

	tests := []struct {
		name   string
		opts   Opts
		assert func(*tls.Config, error)
	}{
		{
			name: "success",
			opts: Opts{
				tls: tlsOpts{
					domain:           "example.com",
					acmeEmail:        "xx@example.com",
					acmeDirectoryUrl: letsEncryptStagingUrl,
				},
			},
			assert: func(c *tls.Config, err error) {
				attest.Ok(t, err)
				attest.NotZero(t, c)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, err := getTlsConfig(tt.opts, l)
			tt.assert(c, err)
		})
	}
}
