package clientip

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/akshayjshah/attest"
)

func TestClientIPstrategy(t *testing.T) {
	t.Parallel()

	awsMetadataApiPrivateIP := "169.254.169.254" // AWS metadata api IP address.
	publicIP := "93.184.216.34"                  // example.com IP address

	tests := []struct {
		name        string
		updateReq   func(*http.Request) string
		runStrategy func(remoteAddr, headerName string, headers http.Header) string
		assert      func(ip string)
	}{
		{
			name: "remoteAddrStrategy",
			runStrategy: func(remoteAddr, headerName string, headers http.Header) string {
				return directAddrStrategy(remoteAddr)
			},
			assert: func(ip string) {
				attest.NotZero(t, ip)
			},
		},

		{
			name: "singleIPHeaderStrategy/bad-header",
			updateReq: func(req *http.Request) string {
				headerName := xForwardedForHeader
				req.Header.Add(headerName, publicIP)
				return headerName
			},
			runStrategy: func(remoteAddr, headerName string, headers http.Header) string {
				return singleIPHeaderStrategy(headerName, headers)
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
				return singleIPHeaderStrategy(headerName, headers)
			},
			assert: func(ip string) {
				attest.Zero(t, ip)
			},
		},
		{
			name: "singleIPHeaderStrategy/not-private-ip",
			updateReq: func(req *http.Request) string {
				headerName := "Fly-Client-IP"
				req.Header.Add(headerName, publicIP)
				return headerName
			},
			runStrategy: func(remoteAddr, headerName string, headers http.Header) string {
				return singleIPHeaderStrategy(headerName, headers)
			},
			assert: func(ip string) {
				attest.NotZero(t, ip)
				attest.Equal(t, ip, publicIP)
			},
		},
		{
			name: "singleIPHeaderStrategy/not-private-ip-with-port",
			updateReq: func(req *http.Request) string {
				headerName := "Fly-Client-IP"
				req.Header.Add(headerName, fmt.Sprintf("%s:%d", publicIP, 9093))
				return headerName
			},
			runStrategy: func(remoteAddr, headerName string, headers http.Header) string {
				return singleIPHeaderStrategy(headerName, headers)
			},
			assert: func(ip string) {
				attest.NotZero(t, ip)
				attest.Equal(t, ip, publicIP)
			},
		},

		{
			name: "leftmostNonPrivateStrategy/bad-header",
			updateReq: func(req *http.Request) string {
				headerName := "Fly-Client-IP"
				req.Header.Add(headerName, publicIP)
				return headerName
			},
			runStrategy: func(remoteAddr, headerName string, headers http.Header) string {
				return leftmostNonPrivateStrategy(headerName, headers)
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
				return leftmostNonPrivateStrategy(headerName, headers)
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
				return leftmostNonPrivateStrategy(headerName, headers)
			},
			assert: func(ip string) {
				attest.Zero(t, ip)
			},
		},
		{
			name: "leftmostNonPrivateStrategy/not-privateIp-xForwardedForHeader",
			updateReq: func(req *http.Request) string {
				headerName := xForwardedForHeader
				req.Header.Add(headerName, publicIP)
				return headerName
			},
			runStrategy: func(remoteAddr, headerName string, headers http.Header) string {
				return leftmostNonPrivateStrategy(headerName, headers)
			},
			assert: func(ip string) {
				attest.NotZero(t, ip)
				attest.Equal(t, ip, publicIP)
			},
		},
		{
			name: "leftmostNonPrivateStrategy/not-privateIp-forwardedHeader",
			updateReq: func(req *http.Request) string {
				headerName := forwardedHeader
				req.Header.Add(
					headerName,
					// see: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Forwarded#transitioning_from_x-forwarded-for_to_forwarded
					fmt.Sprintf("for=%s", publicIP),
				)
				return headerName
			},
			runStrategy: func(remoteAddr, headerName string, headers http.Header) string {
				return leftmostNonPrivateStrategy(headerName, headers)
			},
			assert: func(ip string) {
				attest.NotZero(t, ip)
				attest.Equal(t, ip, publicIP)
			},
		},

		{
			name: "rightmostNonPrivateStrategy/bad-header",
			updateReq: func(req *http.Request) string {
				headerName := "Fly-Client-IP"
				req.Header.Add(headerName, publicIP)
				return headerName
			},
			runStrategy: func(remoteAddr, headerName string, headers http.Header) string {
				return rightmostNonPrivateStrategy(headerName, headers)
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
				return rightmostNonPrivateStrategy(headerName, headers)
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
				return rightmostNonPrivateStrategy(headerName, headers)
			},
			assert: func(ip string) {
				attest.Zero(t, ip)
			},
		},
		{
			name: "rightmostNonPrivateStrategy/not-privateIp-xForwardedForHeader",
			updateReq: func(req *http.Request) string {
				headerName := xForwardedForHeader
				req.Header.Add(headerName, publicIP)
				return headerName
			},
			runStrategy: func(remoteAddr, headerName string, headers http.Header) string {
				return rightmostNonPrivateStrategy(headerName, headers)
			},
			assert: func(ip string) {
				attest.NotZero(t, ip)
				attest.Equal(t, ip, publicIP)
			},
		},
		{
			name: "rightmostNonPrivateStrategy/not-privateIp-forwardedHeader",
			updateReq: func(req *http.Request) string {
				headerName := forwardedHeader
				req.Header.Add(
					headerName,
					// see: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Forwarded#transitioning_from_x-forwarded-for_to_forwarded
					fmt.Sprintf("for=%s", publicIP),
				)
				return headerName
			},
			runStrategy: func(remoteAddr, headerName string, headers http.Header) string {
				return rightmostNonPrivateStrategy(headerName, headers)
			},
			assert: func(ip string) {
				attest.NotZero(t, ip)
				attest.Equal(t, ip, publicIP)
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
