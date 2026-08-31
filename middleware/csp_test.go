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

const nonceHeader = "CUSTOM-CSP-NONCE-TEST-HEADER"

// echoHandler echos back in the response, the msg that was passed in.
func echoHandler(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(nonceHeader, GetCspNonce(r.Context()))
		fmt.Fprint(w, msg)
	}
}

func TestCsp(t *testing.T) {
	t.Parallel()

	t.Run("middleware succeds", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		domain := "example.com"
		wrappedHandler := csp(echoHandler(msg), domain)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), msg)
	})

	t.Run("header set successfully", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		domain := "example.com"
		wrappedHandler := csp(echoHandler(msg), domain)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		attest.Equal(t, rec.Header().Get(cspHeader), getCsp(domain, res.Header.Get(nonceHeader)))
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		domain := "example.com"
		// for this concurrency test, we have to re-use the same wrappedHandler
		// so that state is shared and thus we can see if there is any state which is not handled correctly.
		wrappedHandler := csp(echoHandler(msg), domain)

		runhandler := func() {
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
		for rN := 0; rN <= 14; rN++ {
			wg.Go(func() {
				runhandler()
			})
		}
		wg.Wait()
	})
}

func TestGetCspNonce(t *testing.T) {
	t.Parallel()

	t.Run("can get nonce", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		domain := "example.com"
		wrappedHandler := csp(echoHandler(msg), domain)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		got := res.Header.Get(nonceHeader)
		attest.NotZero(t, got)
		attest.NotEqual(t, got, cspDefaultNonce)
	})
}
