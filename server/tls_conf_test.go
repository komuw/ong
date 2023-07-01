package server

import (
	"bytes"
	"context"
	"testing"

	"github.com/komuw/ong/log"
	"go.akshayshah.org/attest"
)

func TestGetTlsConfig(t *testing.T) {
	t.Parallel()

	l := log.New(&bytes.Buffer{}, 500)(context.Background())
	o := Opts{
		tls: tlsOpts{
			domain:           "example.com",
			acmeEmail:        "xx@example.com",
			acmeDirectoryUrl: letsEncryptStagingUrl,
		},
	}

	c, err := getTlsConfig(o, l)
	attest.Ok(t, err)
	attest.NotZero(t, c)
}
