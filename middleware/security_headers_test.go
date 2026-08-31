package middleware

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"go.akshayshah.org/attest"
)

func TestSecurityHeaders(t *testing.T) {
	t.Parallel()

	t.Run("headers set successfully", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := securityHeaders(echoHandler(msg))

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		req.TLS = &tls.ConnectionState{}
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)
		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), msg)

		expect := map[string]string{
			permissionsPolicyHeader: "interest-cohort=()",
			xContentOptionsHeader:   "nosniff",
			xFrameHeader:            "DENY",
			corpHeader:              "same-site",
			coopHeader:              "same-origin",
			referrerHeader:          "strict-origin-when-cross-origin",
			stsHeader:               getSts(60 * 24 * time.Hour),
		}

		for k, v := range expect {
			attest.Equal(t, rec.Header().Get(k), v)
		}
		attest.Zero(t, rec.Header().Get(cspHeader))
	})

	t.Run("strict transport security omitted without TLS", func(t *testing.T) {
		t.Parallel()

		wrappedHandler := securityHeaders(echoHandler("hello"))
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		attest.Zero(t, rec.Header().Get(stsHeader))
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := securityHeaders(echoHandler(msg))

		runHandler := func() {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
			wrappedHandler.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)
			attest.Equal(t, res.StatusCode, http.StatusOK)
			attest.Equal(t, string(rb), msg)
		}

		wg := &sync.WaitGroup{}
		for range 15 {
			wg.Go(runHandler)
		}
		wg.Wait()
	})
}
