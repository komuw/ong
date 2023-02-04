// Package client provides a HTTP client implementation.
// This client is opinionated and comes with good defaults.
package client

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"os"
	"syscall"
	"time"

	"github.com/komuw/ong/log"
	"golang.org/x/exp/slog"
)

const (
	logIDHeader = string(log.CtxKey)
	errPrefix   = "ong/client:"
)

// Most of the code here is insipired by(or taken from):
//   (a) https://www.agwa.name/blog/post/preventing_server_side_request_forgery_in_golang whose license(CC0 Public Domain) can be found here: https://creativecommons.org/publicdomain/zero/1.0
//   (b) https://www.agwa.name/blog/post/preventing_server_side_request_forgery_in_golang/media/ipaddress.go
// as of 9th/september/2022
//

// Safe creates a http client that has some good defaults & is safe from server-side request forgery (SSRF).
// It also logs requests and responses using [log.Logger]
func Safe(l *slog.Logger) *http.Client {
	return new(true, l)
}

// Unsafe creates a http client that has some good defaults & is NOT safe from server-side request forgery (SSRF).
// It also logs requests and responses using [log.Logger]
func Unsafe(l *slog.Logger) *http.Client {
	return new(false, l)
}

// new creates a client. Use [Safe] or [Unsafe] instead.
func new(ssrfSafe bool, l *slog.Logger) *http.Client {
	// The wikipedia monitoring dashboards are public: https://grafana.wikimedia.org/?orgId=1
	// In there we can see that the p95 response times for http GET requests is ~700ms: https://grafana.wikimedia.org/d/RIA1lzDZk/application-servers-red?orgId=1
	// and the p95 response times for http POST requests is ~3seconds:
	// Thus, we set the timeout to be twice that.
	timeout := 3 * 2 * time.Second

	dialer := &net.Dialer{
		// Using Dialer.ControlContext instead of Dialer.Control allows;
		// - propagation of logging contexts, metric context or other metadata down to the callback.
		// - cancellation if the callback potentially does I/O.
		ControlContext: ssrfSocketControl(ssrfSafe),
		// see: net.DefaultResolver
		Resolver: &net.Resolver{
			// Prefer Go's built-in DNS resolver.
			PreferGo: true,
		},
		// The timeout and keep-alive in the default http.DefaultTransport are 30seconds.
		// see; http.DefaultTransport
		Timeout:   timeout,
		KeepAlive: timeout,
	}

	transport := &http.Transport{
		// see: http.DefaultTransport
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       3 * timeout,
		TLSHandshakeTimeout:   timeout,
		ExpectContinueTimeout: 1 * time.Second,
	}

	lr := &loggingRT{
		RoundTripper: transport,
		l:            l.With("pid", os.Getpid()),
	}

	return &http.Client{
		Transport: lr,
		Timeout:   timeout,
	}
}

// loggingRT is a [http.RoundTripper] that logs requests and responses.
type loggingRT struct {
	l *slog.Logger
	http.RoundTripper
}

func (lr *loggingRT) RoundTrip(req *http.Request) (res *http.Response, err error) {
	ctx := req.Context()
	start := time.Now()
	defer func() {
		l := lr.l.WithContext(ctx)
		msg := "http_client"
		// TODO: (komuw), check if using []any{} is okay.
		flds := []any{
			"method", req.Method,
			"url", req.URL.Redacted(),
			"durationMS", time.Since(start).Milliseconds(),
		}

		if err != nil {
			l.Error(msg, err, flds...)
		} else {
			extra := []any{
				"code", res.StatusCode,
				"status", res.Status,
			}
			flds = append(flds, extra...)
			l.Info(msg, flds...)
		}
	}()

	// TODO: (komuw) should we use l.logID if id from `log.GetId` did not come from ctx.
	id, _ := log.GetId(ctx)
	req.Header.Set(logIDHeader, id)

	return lr.RoundTripper.RoundTrip(req)
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
