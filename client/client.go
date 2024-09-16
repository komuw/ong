// Package client provides a HTTP client implementation.
// This client is opinionated and comes with good defaults.
package client

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"syscall"
	"time"

	"github.com/komuw/ong/internal/octx"
	"github.com/komuw/ong/log"
)

const (
	logIDHeader = string(octx.LogCtxKey)
	errPrefix   = "ong/client:"
	// The wikipedia monitoring dashboards are public: https://grafana.wikimedia.org/?orgId=1
	// In there we can see that the p95 response times for http GET requests is ~700ms: https://grafana.wikimedia.org/d/RIA1lzDZk/application-servers-red?orgId=1
	// and the p95 response times for http POST requests is ~3seconds:
	// Thus, we set the timeout to be twice that.
	defaultTimeout = 2 * 3 * time.Second
)

// Some of the code here is inspired by(or taken from):
//   (a) https://www.agwa.name/blog/post/preventing_server_side_request_forgery_in_golang whose license(CC0 Public Domain) can be found here: https://creativecommons.org/publicdomain/zero/1.0
//   (b) https://www.agwa.name/blog/post/preventing_server_side_request_forgery_in_golang/media/ipaddress.go
//   (c) https://dropbox.tech/security/bug-bounty-program-ssrf-attack
// as of 9th/september/2022
//

// Safe creates a http client that has some good defaults & is safe from server-side request forgery (SSRF).
// It also logs requests and responses using [log.Logger]
// The timeout is optional.
func Safe(l *slog.Logger, timeout ...time.Duration) *http.Client {
	t := defaultTimeout
	if len(timeout) > 0 {
		t = timeout[0]
	}
	return new(true, t, l)
}

// Unsafe creates a http client that has some good defaults & is NOT safe from server-side request forgery (SSRF).
// It also logs requests and responses using [log.Logger]
// The timeout is optional
func Unsafe(l *slog.Logger, timeout ...time.Duration) *http.Client {
	t := defaultTimeout
	if len(timeout) > 0 {
		t = timeout[0]
	}
	return new(false, t, l)
}

// new creates a client. Use [Safe] or [Unsafe] instead.
func new(ssrfSafe bool, timeout time.Duration, l *slog.Logger) *http.Client {
	dialer := &net.Dialer{
		// Using Dialer.ControlContext instead of Dialer.Control allows;
		// - propagation of logging contexts, metric context or other metadata down to the callback.
		// - cancellation if the callback potentially does I/O.
		//
		// ControlContext is called after creating the network connection but before actually dialing.
		// Thus the Safe http client is still vulnerable to ssrf attacks in:
		// (a) http redirection: Since we only validate the initial request, an attacker can redirect it to an internal address and bypass validation of subsequent requests.
		// (b) dns rebinding: An attacker can return a safe IP in the first DNS lookup and a private IP in the second lookup to bypass validation.
		// see:
		//  (i) https://dropbox.tech/security/bug-bounty-program-ssrf-attack
		//  (ii) https://github.com/komuw/ong/issues/221
		ControlContext: ssrfSocketControl(ssrfSafe),
		// see: net.DefaultResolver
		Resolver: &net.Resolver{
			// Prefer Go's built-in DNS resolver.
			PreferGo: true,
		},
		// Timeout is the maximum amount of time a dial will wait for a connect to complete.
		// The timeout and keep-alive in the default http.DefaultTransport are 30seconds.
		// see; http.DefaultTransport
		Timeout: timeout,
		// KeepAlive is interval between keep-alive probes.
		KeepAlive: 3 * timeout,
	}

	transport := &http.Transport{
		// see: http.DefaultTransport
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       5 * timeout,
		TLSHandshakeTimeout:   timeout,
		ExpectContinueTimeout: (timeout / 5),
	}

	lr := &loggingRT{transport, l}

	return &http.Client{
		Transport: lr,
		Timeout:   timeout,
	}
}

// loggingRT is a [http.RoundTripper] that logs requests and responses.
type loggingRT struct {
	*http.Transport
	l *slog.Logger
}

func (lr *loggingRT) RoundTrip(req *http.Request) (res *http.Response, err error) {
	ctx := req.Context()
	start := time.Now()
	defer func() {
		msg := "http_client"
		flds := []any{
			"method", req.Method,
			"url", req.URL.Redacted(),
			"durationMS", time.Since(start).Milliseconds(),
		}

		if err != nil {
			extra := []any{"err", err}
			flds = append(flds, extra...)
			lr.l.ErrorContext(ctx, msg, flds...)
		} else {
			extra := []any{
				"code", res.StatusCode,
				"status", res.Status,
			}
			flds = append(flds, extra...)
			lr.l.InfoContext(ctx, msg, flds...)
		}
	}()

	req.Header.Set(logIDHeader, log.GetId(ctx))

	return lr.Transport.RoundTrip(req)
}

func ssrfSocketControl(ssrfSafe bool) func(ctx context.Context, network, address string, c syscall.RawConn) error {
	if !ssrfSafe {
		return nil
	}

	return func(ctx context.Context, network, address string, c syscall.RawConn) error {
		if !(network == "tcp4" || network == "tcp6") {
			return fmt.Errorf("%s %s is not a safe network type", errPrefix, network)
		}

		if err := isSafeAddress(address); err != nil {
			return err
		}

		return nil
	}
}

func isSafeAddress(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}

	addr, err := netip.ParseAddr(host)
	if err != nil {
		return err
	}

	if addr.IsLoopback() {
		return fmt.Errorf("%s address %s IsLoopback", errPrefix, addr)
	}
	if addr.IsLinkLocalUnicast() {
		return fmt.Errorf("%s address %s IsLinkLocalUnicast", errPrefix, addr)
	}
	if addr.IsPrivate() {
		return fmt.Errorf("%s address %s IsPrivate", errPrefix, addr)
	}

	return nil
}
