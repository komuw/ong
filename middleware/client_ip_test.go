package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"go.akshayshah.org/attest"
)

const (
	xForwardedForHeader = "X-Forwarded-For"
	forwardedHeader     = "Forwarded"
	proxyHeader         = "PROXY"
)

func someClientIpHandler(msg string) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			ip := ClientIP(r)
			res := fmt.Sprintf("message: %s, ip: %s", msg, ip)
			fmt.Fprint(w, res)
		},
	)
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
			strategy ClientIPstrategy
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
			{
				name:     "ProxyStrategy",
				strategy: ProxyStrategy,
				req: func() *http.Request {
					r := httptest.NewRequest(http.MethodGet, "/someUri", nil)
					r.Header.Add(proxyHeader,
						// https://docs.aws.amazon.com/elasticloadbalancing/latest/classic/enable-proxy-protocol.html
						fmt.Sprintf("PROXY TCP4 %s 203.0.113.7 35646 80\r\n", publicIP),
					)
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
