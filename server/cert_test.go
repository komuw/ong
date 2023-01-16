package server

import (
	"bytes"
	"os"
	"testing"

	"github.com/akshayjshah/attest"
	"github.com/komuw/ong/log"
)

func TestCreateDevCertKey(t *testing.T) {
	t.Parallel()

	if os.Getenv("GITHUB_ACTIONS") != "" {
		// CreateDevCertKey() fails in github actions with error: `panic: open /home/runner/ong/rootCA_key.pem: permission denied`
		return
	}

	certPath, keyPath := certKeyPaths()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		os.Remove(certPath)
		os.Remove(keyPath)

		l := log.New(&bytes.Buffer{}, 500)
		_, _ = CreateDevCertKey(l)

		_, err := os.Stat(certPath)
		attest.Ok(t, err)

		_, err = os.Stat(keyPath)
		attest.Ok(t, err)
	})
}
