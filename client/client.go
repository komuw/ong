// Package client provides a HTTP client implementation.
// This client is opinionated and comes with good defaults.
package client

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"syscall"
	"time"

	"github.com/komuw/ong/log"
)

// Most of the code here is insipired by(or taken from):
//   (a) https://www.agwa.name/blog/post/preventing_server_side_request_forgery_in_golang whose license(CC0 Public Domain) can be found here: https://creativecommons.org/publicdomain/zero/1.0
//   (b) https://www.agwa.name/blog/post/preventing_server_side_request_forgery_in_golang/media/ipaddress.go
// as of 9th/september/2022
//

// Safe creates a client that is safe from the server-side request forgery (SSRF) security vulnerability.
func Safe(l log.Logger) *Client {
	return new(true, l)
}

// Unsafe creates a client that is NOT safe from the server-side request forgery (SSRF) security vulnerability.
func Unsafe(l log.Logger) *Client {
	return new(false, l)
}

// Client is a [http.Client] that has some good defaults. It also logs requests and responses using [log.Logger]
//
// Use either [Safe] or [Unsafe] to get a valid client.
//
// Clients should be reused instead of created as needed. Clients are safe for concurrent use by multiple goroutines.
//
// see [http.Client]
type Client struct {
	cli *http.Client
	l   log.Logger
}

// new creates a client. Use [Safe] or [Unsafe] instead.
func new(ssrfSafe bool, l log.Logger) *Client {
	// The wikipedia monitoring dashboards are public: https://grafana.wikimedia.org/?orgId=1
	// In there we can see that the p95 response times for http GET requests is ~700ms: https://grafana.wikimedia.org/d/RIA1lzDZk/application-servers-red?orgId=1
	// and the p95 response times for http POST requests is ~3seconds:
	// Thus, we set the timeout to be twice that.
	timeout := 3 * 2 * time.Second

	dialer := &net.Dialer{
		Control: ssrfSocketControl(ssrfSafe),
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
	cli := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	return &Client{
		cli: cli,
		l:   l.WithFields(log.F{"pid": os.Getpid()}),
	}
}

// CloseIdleConnections closes any connections on its Transport which were previously connected from previous requests but are now sitting idle in a "keep-alive" state.
//
// It does not interrupt any connections currently in use.
//
// see [http.Client.CloseIdleConnections]
func (c *Client) CloseIdleConnections() {
	c.cli.CloseIdleConnections()
}

// Do sends an HTTP request and returns an HTTP response, following policy (such as redirects, cookies, auth) as configured on the client.
//
// see [http.Client.Do]
func (c *Client) Do(ctx context.Context, req *http.Request) (resp *http.Response, err error) {
	end := c.log(ctx, req.URL.EscapedPath(), req.Method)
	defer func() {
		end(resp, err)
	}()

	return c.cli.Do(req)
}

// Get issues a GET to the specified URL.
//
// see [http.Client.Get]
func (c *Client) Get(ctx context.Context, url string) (resp *http.Response, err error) {
	end := c.log(ctx, url, "GET")
	defer func() {
		end(resp, err)
	}()

	return c.cli.Get(url)
}

// Head issues a HEAD to the specified URL.
//
// see [http.Client.Head]
func (c *Client) Head(ctx context.Context, url string) (resp *http.Response, err error) {
	end := c.log(ctx, url, "HEAD")
	defer func() {
		end(resp, err)
	}()

	return c.cli.Head(url)
}

// Post issues a POST to the specified URL.
//
// see [http.Client.Post]
func (c *Client) Post(ctx context.Context, url, contentType string, body io.Reader) (resp *http.Response, err error) {
	end := c.log(ctx, url, "POST")
	defer func() {
		end(resp, err)
	}()

	return c.cli.Post(url, contentType, body)
}

// PostForm issues a POST to the specified URL, with data's keys and values URL-encoded as the request body.
//
// see [http.Client.PostForm]
func (c *Client) PostForm(ctx context.Context, url string, data url.Values) (resp *http.Response, err error) {
	end := c.log(ctx, url, "POST")
	defer func() {
		end(resp, err)
	}()

	return c.cli.PostForm(url, data)
}

func (c *Client) log(ctx context.Context, url, method string) func(resp *http.Response, err error) {
	l := c.l.WithCtx(ctx)
	l.Info(log.F{
		"msg":     "http_client",
		"process": "request",
		"method":  method,
		"url":     url,
	})

	start := time.Now()

	return func(resp *http.Response, err error) {
		flds := log.F{
			"msg":        "http_client",
			"process":    "response",
			"method":     "GET",
			"url":        url,
			"durationMS": time.Since(start).Milliseconds(),
		}
		if resp != nil {
			flds["code"] = resp.StatusCode
			flds["status"] = resp.Status

			if resp.StatusCode >= http.StatusBadRequest {
				// both client and server errors.
				l.Error(err, flds)
			} else {
				l.Info(flds)
			}
		} else {
			l.Error(err, flds)
		}
	}
}

const errPrefix = "ong/client:"

func ssrfSocketControl(ssrfSafe bool) func(network, address string, conn syscall.RawConn) error {
	if !ssrfSafe {
		return nil
	}

	return func(network, address string, conn syscall.RawConn) error {
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
