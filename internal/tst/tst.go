// Package tst implements some common test functionality needed across ong.
package tst

import (
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"

	"go.akshayshah.org/attest"
)

// CustomServer starts a server at a predetermined port.
// It's upto callers to close the server.
func CustomServer(t attest.TB, h http.Handler, domain string, httpsPort uint16) *httptest.Server {
	t.Helper()

	ts := httptest.NewUnstartedServer(h)
	ts.Listener.Close()

	l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", domain, httpsPort))
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
