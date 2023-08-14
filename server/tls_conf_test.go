package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"testing"

	"github.com/komuw/ong/log"
	"go.akshayshah.org/attest"
)

func TestGetTlsConfig(t *testing.T) {
	t.Parallel()

	l := log.New(context.Background(), &bytes.Buffer{}, 500)

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
		{
			name: "bad domain",
			opts: Opts{
				tls: tlsOpts{
					domain:           "example.*org",
					acmeEmail:        "xx@example.com",
					acmeDirectoryUrl: letsEncryptStagingUrl,
				},
			},
			assert: func(c *tls.Config, err error) {
				attest.Error(t, err)
				attest.Zero(t, c)
			},
		},
		{
			name: "non nil pool with no tls args",
			opts: Opts{
				tls: tlsOpts{
					domain:                "example.com",
					acmeEmail:             "",
					acmeDirectoryUrl:      letsEncryptStagingUrl,
					clientCertificatePool: &x509.CertPool{},
				},
			},
			assert: func(c *tls.Config, err error) {
				attest.Error(t, err)
				attest.Zero(t, c)
			},
		},
		{
			name: "cert pool success",
			opts: Opts{
				tls: tlsOpts{
					domain:                "example.com",
					acmeEmail:             "xx@example.com",
					acmeDirectoryUrl:      letsEncryptStagingUrl,
					clientCertificatePool: &x509.CertPool{},
				},
			},
			assert: func(c *tls.Config, err error) {
				attest.Ok(t, err)
				attest.NotZero(t, c)
				attest.Equal(t, c.ClientAuth, tls.RequireAndVerifyClientCert)
			},
		},
		{
			name: "cert pool from system success",
			opts: Opts{
				tls: tlsOpts{
					domain:           "example.com",
					acmeEmail:        "xx@example.com",
					acmeDirectoryUrl: letsEncryptStagingUrl,
					clientCertificatePool: func() *x509.CertPool {
						p, err := x509.SystemCertPool()
						attest.Ok(t, err)
						return p
					}(),
				},
			},
			assert: func(c *tls.Config, err error) {
				attest.Ok(t, err)
				attest.NotZero(t, c)
				attest.Equal(t, c.ClientAuth, tls.RequireAndVerifyClientCert)
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
