package server

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/komuw/ong/log"

	"go.akshayshah.org/attest"
)

func TestCreateDevCertKey(t *testing.T) {
	t.Parallel()

	certPath, keyPath := certKeyPaths()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		os.Remove(certPath)
		os.Remove(keyPath)

		l := log.New(context.Background(), &bytes.Buffer{}, 500)
		_, _ = createDevCertKey(l)

		_, err := os.Stat(certPath)
		attest.Ok(t, err)

		_, err = os.Stat(keyPath)
		attest.Ok(t, err)
	})
}
