// Package tst implements some common test functionality needed across ong.
package tst

import (
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"time"

	"go.akshayshah.org/attest"
)

// TlsServer starts a test TLS server at a predetermined port and returns it.
// It's upto callers to close the server.
func TlsServer(t attest.TB, h http.Handler, domain string, httpsPort uint16) *httptest.Server {
	t.Helper()

	ts := httptest.NewUnstartedServer(h)
	err := ts.Listener.Close()
	attest.Ok(t, err)

	l, err := net.Listen("tcp", net.JoinHostPort(domain, fmt.Sprintf("%d", httpsPort)))
	attest.Ok(t, err)

	ts.Listener = l
	ts.StartTLS()

	return ts
}

// GetPort returns a random port.
// The idea is that different tests should run on different independent ports to avoid collisions.
func GetPort() uint16 {
	r := rand.Intn(10_000) + 1
	p := math.MaxUint16 - uint16(r)
	return p
}

// SecretKey returns a secret key that is valid and can be used in tests.
func SecretKey() string {
	return "super-h@rd-Pa$1word"
}

// Ping waits for port to be open, it fails after a number of given seconds.
func Ping(t attest.TB, port uint16) {
	t.Helper()

	var err error
	count := 0
	maxCount := 12
	defer func() {
		attest.Ok(t, err)
	}()

	for {
		count = count + 1
		time.Sleep(1 * time.Second)
		_, err = net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 1*time.Second)
		if err == nil {
			break
		}

		if count > maxCount {
			err = fmt.Errorf("ping max count(%d) reached: %w", maxCount, err)
			break
		}
	}
}
