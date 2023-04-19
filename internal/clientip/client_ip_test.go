package clientip

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/akshayjshah/attest"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	// call flag.Parse() here if TestMain uses flags
	goleak.VerifyTestMain(m)
}

func TestClientIPstrategy(t *testing.T) {
	t.Parallel()

	awsMetadataApiPrivateIP := "169.254.169.254"       // AWS metadata api IP address.
	publicIPv4 := "93.184.216.34"                      // example.com IP4 address
	publicIPv6 := "2606:2800:220:1:248:1893:25c8:1946" // example.com IP6 address

	tests := []struct {
		name        string
		updateReq   func(*http.Request) string
		runStrategy func(remoteAddr, headerName string, headers http.Header) string
		assert      func(ip string)
	}{
		{
			name: "remoteAddrStrategy",
			runStrategy: func(remoteAddr, headerName string, headers http.Header) string {
				return DirectAddress(remoteAddr)
			},
			assert: func(ip string) {
				attest.NotZero(t, ip)
			},
		},

		{
			name: "singleIPHeaderStrategy/bad-header",
			updateReq: func(req *http.Request) string {
				headerName := xForwardedForHeader
				req.Header.Add(headerName, publicIPv4)
				return headerName
			},
			runStrategy: func(remoteAddr, headerName string, headers http.Header) string {
				return SingleIPHeader(headerName, headers)
			},
			assert: func(ip string) {
				attest.Zero(t, ip)
			},
		},
		{
			name: "singleIPHeaderStrategy/private-ip",
			updateReq: func(req *http.Request) string {
				headerName := "Fly-Client-IP"
				req.Header.Add(headerName, awsMetadataApiPrivateIP)
				return headerName
			},
			runStrategy: func(remoteAddr, headerName string, headers http.Header) string {
				return SingleIPHeader(headerName, headers)
			},
			assert: func(ip string) {
				attest.Zero(t, ip)
			},
		},
		{
			name: "singleIPHeaderStrategy/not-private-ip",
			updateReq: func(req *http.Request) string {
				headerName := "Fly-Client-IP"
				req.Header.Add(headerName, publicIPv4)
				return headerName
			},
			runStrategy: func(remoteAddr, headerName string, headers http.Header) string {
				return SingleIPHeader(headerName, headers)
			},
			assert: func(ip string) {
				attest.NotZero(t, ip)
				attest.Equal(t, ip, publicIPv4)
			},
		},
		{
			name: "singleIPHeaderStrategy/not-private-ip-with-port",
			updateReq: func(req *http.Request) string {
				headerName := "Fly-Client-IP"
				req.Header.Add(headerName, fmt.Sprintf("%s:%d", publicIPv4, 9093))
				return headerName
			},
			runStrategy: func(remoteAddr, headerName string, headers http.Header) string {
				return SingleIPHeader(headerName, headers)
			},
			assert: func(ip string) {
				attest.NotZero(t, ip)
				attest.Equal(t, ip, publicIPv4)
			},
		},

		{
			name: "leftmostNonPrivateStrategy/bad-header",
			updateReq: func(req *http.Request) string {
				headerName := "Fly-Client-IP"
				req.Header.Add(headerName, publicIPv4)
				return headerName
			},
			runStrategy: func(remoteAddr, headerName string, headers http.Header) string {
				return Leftmost(headers)
			},
			assert: func(ip string) {
				attest.Zero(t, ip)
			},
		},
		{
			name: "leftmostNonPrivateStrategy/privateIp-xForwardedForHeader",
			updateReq: func(req *http.Request) string {
				headerName := xForwardedForHeader
				req.Header.Add(headerName, awsMetadataApiPrivateIP)
				return headerName
			},
			runStrategy: func(remoteAddr, headerName string, headers http.Header) string {
				return Leftmost(headers)
			},
			assert: func(ip string) {
				attest.Zero(t, ip)
			},
		},
		{
			name: "leftmostNonPrivateStrategy/privateIp-forwardedHeader",
			updateReq: func(req *http.Request) string {
				headerName := forwardedHeader
				req.Header.Add(
					headerName,
					// see: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Forwarded#transitioning_from_x-forwarded-for_to_forwarded
					fmt.Sprintf("for=%s", awsMetadataApiPrivateIP),
				)
				return headerName
			},
			runStrategy: func(remoteAddr, headerName string, headers http.Header) string {
				return Leftmost(headers)
			},
			assert: func(ip string) {
				attest.Zero(t, ip)
			},
		},
		{
			name: "leftmostNonPrivateStrategy/not-privateIp-xForwardedForHeader",
			updateReq: func(req *http.Request) string {
				headerName := xForwardedForHeader
				req.Header.Add(headerName, publicIPv4)
				return headerName
			},
			runStrategy: func(remoteAddr, headerName string, headers http.Header) string {
				return Leftmost(headers)
			},
			assert: func(ip string) {
				attest.NotZero(t, ip)
				attest.Equal(t, ip, publicIPv4)
			},
		},
		{
			name: "leftmostNonPrivateStrategy/not-privateIp-forwardedHeader",
			updateReq: func(req *http.Request) string {
				headerName := forwardedHeader
				req.Header.Add(
					headerName,
					// see: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Forwarded#transitioning_from_x-forwarded-for_to_forwarded
					fmt.Sprintf("for=%s", publicIPv4),
				)
				return headerName
			},
			runStrategy: func(remoteAddr, headerName string, headers http.Header) string {
				return Leftmost(headers)
			},
			assert: func(ip string) {
				attest.NotZero(t, ip)
				attest.Equal(t, ip, publicIPv4)
			},
		},

		{
			name: "rightmostNonPrivateStrategy/bad-header",
			updateReq: func(req *http.Request) string {
				headerName := "Fly-Client-IP"
				req.Header.Add(headerName, publicIPv4)
				return headerName
			},
			runStrategy: func(remoteAddr, headerName string, headers http.Header) string {
				return Rightmost(headers)
			},
			assert: func(ip string) {
				attest.Zero(t, ip)
			},
		},
		{
			name: "rightmostNonPrivateStrategy/privateIp-xForwardedForHeader",
			updateReq: func(req *http.Request) string {
				headerName := xForwardedForHeader
				req.Header.Add(headerName, awsMetadataApiPrivateIP)
				return headerName
			},
			runStrategy: func(remoteAddr, headerName string, headers http.Header) string {
				return Rightmost(headers)
			},
			assert: func(ip string) {
				attest.Zero(t, ip)
			},
		},
		{
			name: "rightmostNonPrivateStrategy/privateIp-forwardedHeader",
			updateReq: func(req *http.Request) string {
				headerName := forwardedHeader
				req.Header.Add(
					headerName,
					// see: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Forwarded#transitioning_from_x-forwarded-for_to_forwarded
					fmt.Sprintf("for=%s", awsMetadataApiPrivateIP),
				)
				return headerName
			},
			runStrategy: func(remoteAddr, headerName string, headers http.Header) string {
				return Rightmost(headers)
			},
			assert: func(ip string) {
				attest.Zero(t, ip)
			},
		},
		{
			name: "rightmostNonPrivateStrategy/not-privateIp-xForwardedForHeader",
			updateReq: func(req *http.Request) string {
				headerName := xForwardedForHeader
				req.Header.Add(headerName, publicIPv4)
				return headerName
			},
			runStrategy: func(remoteAddr, headerName string, headers http.Header) string {
				return Rightmost(headers)
			},
			assert: func(ip string) {
				attest.NotZero(t, ip)
				attest.Equal(t, ip, publicIPv4)
			},
		},
		{
			name: "rightmostNonPrivateStrategy/not-privateIp-forwardedHeader",
			updateReq: func(req *http.Request) string {
				headerName := forwardedHeader
				req.Header.Add(
					headerName,
					// see: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Forwarded#transitioning_from_x-forwarded-for_to_forwarded
					fmt.Sprintf("for=%s", publicIPv4),
				)
				return headerName
			},
			runStrategy: func(remoteAddr, headerName string, headers http.Header) string {
				return Rightmost(headers)
			},
			assert: func(ip string) {
				attest.NotZero(t, ip)
				attest.Equal(t, ip, publicIPv4)
			},
		},

		{
			name: "ProxyHeader/empty",
			updateReq: func(req *http.Request) string {
				headerName := proxyHeader
				req.Header.Add(headerName, "")
				return headerName
			},
			runStrategy: func(remoteAddr, headerName string, headers http.Header) string {
				return ProxyHeader(headers)
			},
			assert: func(ip string) {
				attest.Zero(t, ip)
			},
		},
		{
			name: "ProxyHeader/unknown",
			updateReq: func(req *http.Request) string {
				headerName := proxyHeader
				req.Header.Add(headerName, "PROXY UNKNOWN\r\n")
				return headerName
			},
			runStrategy: func(remoteAddr, headerName string, headers http.Header) string {
				return ProxyHeader(headers)
			},
			assert: func(ip string) {
				attest.Zero(t, ip)
			},
		},
		{
			name: "ProxyHeader/ipv4",
			updateReq: func(req *http.Request) string {
				headerName := proxyHeader
				req.Header.Add(headerName,
					// https://docs.aws.amazon.com/elasticloadbalancing/latest/classic/enable-proxy-protocol.html
					fmt.Sprintf("PROXY TCP4 %s 203.0.113.7 35646 80\r\n", publicIPv4),
				)
				return headerName
			},
			runStrategy: func(remoteAddr, headerName string, headers http.Header) string {
				return ProxyHeader(headers)
			},
			assert: func(ip string) {
				attest.NotZero(t, ip)
				attest.Equal(t, ip, publicIPv4)
			},
		},
		{
			name: "ProxyHeader/ipv6",
			updateReq: func(req *http.Request) string {
				headerName := proxyHeader
				req.Header.Add(headerName,
					// https://docs.aws.amazon.com/elasticloadbalancing/latest/classic/enable-proxy-protocol.html
					fmt.Sprintf("PROXY TCP6 %s 2001:DB8::12f:8baa:eafc:ce29:6b2e 35646 80\r\n", publicIPv6),
				)
				return headerName
			},
			runStrategy: func(remoteAddr, headerName string, headers http.Header) string {
				return ProxyHeader(headers)
			},
			assert: func(ip string) {
				attest.NotZero(t, ip)
				attest.Equal(t, ip, publicIPv6)
			},
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			headerName := ""
			req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
			if tt.updateReq != nil {
				headerName = tt.updateReq(req)
			}

			remoteAddr := req.RemoteAddr
			headers := req.Header
			ip := tt.runStrategy(remoteAddr, headerName, headers)

			tt.assert(ip)
		})
	}
}
