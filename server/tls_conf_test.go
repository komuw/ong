package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"testing"

	"github.com/komuw/ong/config"
	"github.com/komuw/ong/internal/tst"
	"github.com/komuw/ong/log"
	"go.akshayshah.org/attest"
)

func TestGetTlsConfig(t *testing.T) {
	t.Parallel()

	l := log.New(context.Background(), &bytes.Buffer{}, 500)

	tests := []struct {
		name   string
		opts   func() config.Opts
		assert func(*tls.Config, error)
	}{
		{
			name: "success",
			opts: func() config.Opts { return config.DevOpts(l, tst.SecretKey()) },
			assert: func(c *tls.Config, err error) {
				attest.Ok(t, err)
				attest.NotZero(t, c)
			},
		},
		{
			name: "bad domain",
			opts: func() config.Opts {
				o := config.WithOpts("example.org", 65081, tst.SecretKey(), config.DirectIpStrategy, l)
				// If you pass a bad domain to `config.WithOpts`, it will panic since it validates domain.
				// So we have to do it like this to get an opt with a bad domain.
				o.Domain = "example.*org"
				o.Tls.Hosts = []string{"example.*org"}
				return o
			},
			assert: func(c *tls.Config, err error) {
				attest.Error(t, err)
				attest.Zero(t, c)
			},
		},
		{
			name: "non nil pool with no tls args",
			opts: func() config.Opts {
				o := config.AcmeOpts("example.com", tst.SecretKey(), config.DirectIpStrategy, l, "", []string{"example.com"}, config.LetsEncryptStagingUrl)
				o.Tls.ClientCertificatePool = &x509.CertPool{}
				return o
			},
			assert: func(c *tls.Config, err error) {
				attest.Error(t, err)
				attest.Zero(t, c)
			},
		},
		{
			name: "cert pool success",
			opts: func() config.Opts {
				o := config.AcmeOpts("example.com", tst.SecretKey(), config.DirectIpStrategy, l, "xx@example.com", []string{"example.com"}, config.LetsEncryptStagingUrl)
				o.Tls.ClientCertificatePool = &x509.CertPool{}
				return o
			},
			assert: func(c *tls.Config, err error) {
				attest.Ok(t, err)
				attest.NotZero(t, c)
				attest.Equal(t, c.ClientAuth, tls.RequireAndVerifyClientCert)
			},
		},
		{
			name: "cert pool from system success",
			opts: func() config.Opts {
				o := config.AcmeOpts("example.com", tst.SecretKey(), config.DirectIpStrategy, l, "xx@example.com", []string{"example.com"}, config.LetsEncryptStagingUrl)
				o.Tls.ClientCertificatePool = func() *x509.CertPool {
					p, err := x509.SystemCertPool()
					attest.Ok(t, err)
					return p
				}()
				return o
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

			c, err := getTlsConfig(tt.opts())
			tt.assert(c, err)
		})
	}
}
