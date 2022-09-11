// Package client provides HTTP client implementation.
// The client provided in here is opinionated and comes with good defaults.
package client

import (
	"fmt"
	"net"
	"net/http"
	"syscall"
	"time"
)

// Most of the code here is insipired by(or taken from):
//   (a) https://www.agwa.name/blog/post/preventing_server_side_request_forgery_in_golang whose license(CC0 Public Domain) can be found here: https://creativecommons.org/publicdomain/zero/1.0
//   (b) https://www.agwa.name/blog/post/preventing_server_side_request_forgery_in_golang/media/ipaddress.go
// as of 9th/september/2022

// TODO: maybe we need a global var similar to [http.DefaultClient]
//       or maybe use a func that uses sync.once

// TODO: docs.
func New(ssrfSafe bool) *http.Client {
	timeout := 30 * time.Second
	dialer := &net.Dialer{
		Control: ssrfSocketControl(ssrfSafe),
		// this timeout and keep-alive are similar to the ones used by stdlib.
		Timeout:   timeout,
		KeepAlive: timeout,
		DualStack: true,
	}

	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       3 * timeout,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	cli := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	return cli
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

func ipv4Net(a, b, c, d byte, subnetPrefixLen int) net.IPNet {
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
