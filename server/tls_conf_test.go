package server

import (
	"testing"

	"go.akshayshah.org/attest"
)

func TestGetTlsConfig(t *testing.T) {
	t.Parallel()

	o := Opts{
		tls: tlsOpts{
			domain:           "example.com",
			acmeEmail:        "xx@example.com",
			acmeDirectoryUrl: letsEncryptStagingUrl,
		},
	}

	c, err := getTlsConfig(o)
	attest.Ok(t, err)
	attest.NotZero(t, c)
}
