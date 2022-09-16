// Package client provides HTTP client implementation.
// The client provided in here is opinionated and comes with good defaults.
package client

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
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

// SafeClient creates a client that is safe from Server-side request forgery (SSRF) security vulnerability.
func SafeClient(l log.Logger) *Client {
	return new(true, l)
}

// UnsafeClient creates a client that is NOT safe from Server-side request forgery (SSRF) security vulnerability.
func UnsafeClient(l log.Logger) *Client {
	return new(false, l)
}

// Client is a [http.Client] that has some good defaults. It also logs requests and responses using [log.Logger]
//
// Use either [SafeClient] or [UnsafeClient] to get a valid client.
//
// Clients should be reused instead of created as needed. Clients are safe for concurrent use by multiple goroutines.
//
// see [http.Client]
type Client struct {
	cli *http.Client
	l   log.Logger
}

// new creates a client. Use [SafeClient] or [UnsafeClient] instead.
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

func ssrfSocketControl(ssrfSafe bool) func(network, address string, conn syscall.RawConn) error {
	if !ssrfSafe {
		return nil
	}

	return func(network, address string, conn syscall.RawConn) error {
		if !(network == "tcp4" || network == "tcp6") {
			return fmt.Errorf("%s is not a safe network type", network)
		}

		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return fmt.Errorf("%s is not a valid host/port pair: %s", address, err)
		}

		ipaddress := net.ParseIP(host)
		if ipaddress == nil {
			return fmt.Errorf("%s is not a valid IP address", host)
		}

		if !isPublicIPAddress(ipaddress) {
			return fmt.Errorf("%s is not a public IP address", ipaddress)
		}

		return nil
	}
}

func ipv4Net(a, b, c, d byte, subnetPrefixLen int) net.IPNet { // nolint:unparam
	return net.IPNet{
		IP:   net.IPv4(a, b, c, d),
		Mask: net.CIDRMask(96+subnetPrefixLen, 128),
	}
}

func isIPv6GlobalUnicast(address net.IP) bool {
	globalUnicastIPv6Net := net.IPNet{
		IP:   net.IP{0x20, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		Mask: net.CIDRMask(3, 128),
	}
	return globalUnicastIPv6Net.Contains(address)
}

func isIPv4Reserved(address net.IP) bool {
	reservedIPv4Nets := []net.IPNet{
		ipv4Net(0, 0, 0, 0, 8),       // Current network
		ipv4Net(10, 0, 0, 0, 8),      // Private
		ipv4Net(100, 64, 0, 0, 10),   // RFC6598
		ipv4Net(127, 0, 0, 0, 8),     // Loopback
		ipv4Net(169, 254, 0, 0, 16),  // Link-local
		ipv4Net(172, 16, 0, 0, 12),   // Private
		ipv4Net(192, 0, 0, 0, 24),    // RFC6890
		ipv4Net(192, 0, 2, 0, 24),    // Test, doc, examples
		ipv4Net(192, 88, 99, 0, 24),  // IPv6 to IPv4 relay
		ipv4Net(192, 168, 0, 0, 16),  // Private
		ipv4Net(198, 18, 0, 0, 15),   // Benchmarking tests
		ipv4Net(198, 51, 100, 0, 24), // Test, doc, examples
		ipv4Net(203, 0, 113, 0, 24),  // Test, doc, examples
		ipv4Net(224, 0, 0, 0, 4),     // Multicast
		ipv4Net(240, 0, 0, 0, 4),     // Reserved (includes broadcast / 255.255.255.255)
	}

	for _, reservedNet := range reservedIPv4Nets {
		if reservedNet.Contains(address) {
			return true
		}
	}
	return false
}

func isPublicIPAddress(address net.IP) bool {
	if address.To4() != nil {
		return !isIPv4Reserved(address)
	} else {
		return isIPv6GlobalUnicast(address)
	}
}