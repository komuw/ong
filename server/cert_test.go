package server

import (
	"os"
	"testing"

	"github.com/akshayjshah/attest"
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

		_, _ = CreateDevCertKey()

		_, err := os.Stat(certPath)
		attest.Ok(t, err)

		_, err = os.Stat(keyPath)
		attest.Ok(t, err)
	})
}
