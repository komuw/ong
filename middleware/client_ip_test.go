package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/akshayjshah/attest"
)

func someClientIpHandler(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := GetClientIP(r)
		res := fmt.Sprintf("message: %s, ip: %s", msg, ip)
		fmt.Fprint(w, res)
	}
}

func TestClientIP(t *testing.T) {
	t.Parallel()

	// awsMetadataApiPrivateIP := "169.254.169.254" // AWS metadata api IP address.
	publicIP := "93.184.216.34" // example.com IP address

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := clientIP(someClientIpHandler(msg), DirectIpStrategy)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Subsequence(t, string(rb), msg)
	})

	t.Run("ip is added", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			strategy clientIPstrategy
			req      func() *http.Request
			expected string
		}{
			{
				name:     "DirectIpStrategy",
				strategy: DirectIpStrategy,
				req:      func() *http.Request { return httptest.NewRequest(http.MethodGet, "/someUri", nil) },
			},
			{
				name:     "SingleIpStrategy",
				strategy: SingleIpStrategy("Fly-Client-IP"),
				req: func() *http.Request {
					r := httptest.NewRequest(http.MethodGet, "/someUri", nil)
					r.Header.Add("Fly-Client-IP", publicIP)
					return r
				},
				expected: publicIP,
			},
			{
				name:     "LeftIpStrategy",
				strategy: LeftIpStrategy,
				req: func() *http.Request {
					r := httptest.NewRequest(http.MethodGet, "/someUri", nil)
					r.Header.Add(xForwardedForHeader, publicIP)
					return r
				},
				expected: publicIP,
			},
			{
				name:     "RightIpStrategy",
				strategy: RightIpStrategy,
				req: func() *http.Request {
					r := httptest.NewRequest(http.MethodGet, "/someUri", nil)
					r.Header.Add(xForwardedForHeader, publicIP)
					return r
				},
				expected: publicIP,
			},
		}

		for _, tt := range tests {
			tt := tt

			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				msg := "hello"
				wrappedHandler := clientIP(someClientIpHandler(msg), tt.strategy)
				rec := httptest.NewRecorder()
				req := tt.req()
				wrappedHandler.ServeHTTP(rec, req)

				res := rec.Result()
				defer res.Body.Close()

				rb, err := io.ReadAll(res.Body)
				attest.Ok(t, err)

				attest.Equal(t, res.StatusCode, http.StatusOK)
				attest.Subsequence(t, string(rb), msg)
				attest.Subsequence(t, string(rb), tt.expected)
				fmt.Println("\n\t res: ", string(rb))
			})
		}
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := clientIP(someClientIpHandler(msg), DirectIpStrategy)

		runhandler := func() {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
			wrappedHandler.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)

			attest.Equal(t, res.StatusCode, http.StatusOK)
			attest.Subsequence(t, string(rb), msg)
		}

		wg := &sync.WaitGroup{}
		for rN := 0; rN <= 11; rN++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				runhandler()
			}()
		}
		wg.Wait()
	})
}

// TODO: use table-driven tests.
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

	// t.Run("rightmostNonPrivateStrategy", func(t *testing.T) {
	// 	t.Parallel()

	// 	t.Run("privateIp xForwardedForHeader", func(t *testing.T) {
	// 		t.Parallel()

	// 		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
	// 		headerName := xForwardedForHeader
	// 		hdrVal := awsMetadataApiPrivateIP
	// 		req.Header.Add(headerName, hdrVal)

	// 		ip := rightmostNonPrivateStrategy(headerName, req.Header)
	// 		attest.Zero(t, ip)
	// 	})
	// 	t.Run("privateIp forwardedHeader", func(t *testing.T) {
	// 		t.Parallel()

	// 		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
	// 		headerName := forwardedHeader
	// 		hdrVal := awsMetadataApiPrivateIP
	// 		req.Header.Add(headerName, hdrVal)

	// 		ip := rightmostNonPrivateStrategy(headerName, req.Header)
	// 		attest.Zero(t, ip)
	// 	})
	// 	t.Run("not privateIp xForwardedForHeader", func(t *testing.T) {
	// 		t.Parallel()

	// 		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
	// 		headerName := xForwardedForHeader
	// 		hdrVal := publicIP
	// 		req.Header.Add(headerName, hdrVal)

	// 		ip := rightmostNonPrivateStrategy(headerName, req.Header)
	// 		attest.NotZero(t, ip)
	// 		attest.Equal(t, ip, hdrVal)
	// 		fmt.Println("ip: ", ip, " : ", req.RemoteAddr)
	// 	})
	// 	t.Run("not privateIp forwardedHeader", func(t *testing.T) {
	// 		t.Parallel()

	// 		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
	// 		headerName := xForwardedForHeader
	// 		hdrVal := publicIP
	// 		req.Header.Add(headerName, hdrVal)

	// 		ip := rightmostNonPrivateStrategy(headerName, req.Header)
	// 		attest.NotZero(t, ip)
	// 		attest.Equal(t, ip, hdrVal)
	// 		fmt.Println("ip: ", ip, " : ", req.RemoteAddr)
	// 	})
	// })
}
