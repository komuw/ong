package server

import (
	"testing"

	"go.akshayshah.org/attest"
)

func TestGetTlsConfig(t *testing.T) {
	t.Parallel()

	o := Opts{
		tls: tlsOpts{
			domain: "example.com",
			email:  "xx@example.com",
			url:    letsEncryptStagingUrl,
		},
	}

	c, acmeH, err := getTlsConfig(o)
	attest.Ok(t, err)
	attest.NotZero(t, acmeH)
	attest.NotZero(t, c)
}
