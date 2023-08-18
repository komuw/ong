// Package tst implements some common test functionality needed across ong.
package tst

import (
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/net/nettest"
)

// TlsServer starts a test TLS server at a predetermined port and returns it.
// It's upto callers to close the server.
func TlsServer(h http.Handler, domain string, httpsPort uint16) (*httptest.Server, error) {
	if !testing.Testing() {
		panic("this func should only be called from tests")
	}

	ts := httptest.NewUnstartedServer(h)
	if err := ts.Listener.Close(); err != nil {
		return nil, err
	}

	l, err := net.Listen("tcp", net.JoinHostPort(domain, fmt.Sprintf("%d", httpsPort)))
	if err != nil {
		return nil, err
	}

	ts.Listener = l
	ts.StartTLS()

	return ts, nil
}

// GetPort returns a random port.
// The idea is that different tests should run on different independent ports to avoid collisions.
func GetPort() uint16 {
	if !testing.Testing() {
		panic("this func should only be called from tests")
	}

	{ // Note: There's a possible race condition here.
		// Where this scope gets us a free port, but it becomes used by someone else before we can.
		l, err := nettest.NewLocalListener("tcp")
		if err != nil {
			goto fallback
		}
		defer l.Close()

		addr, ok := l.Addr().(*net.TCPAddr)
		if !ok || (addr == nil) {
			goto fallback
		}

		return uint16(addr.Port)
	}

fallback:
	{
		r := rand.Intn(10_000) + 1
		p := math.MaxUint16 - uint16(r)
		return p
	}
}

// SecretKey returns a secret key that is valid and can be used in tests.
func SecretKey() string {
	if !testing.Testing() {
		panic("this func should only be called from tests")
	}

	return "super-h@rd-Pa$1word"
}

// Ping waits for port to be open, it fails after a number of given seconds.
func Ping(port uint16) error {
	if !testing.Testing() {
		panic("this func should only be called from tests")
	}

	var err error
	count := 0
	maxCount := 12

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

	return err
}
