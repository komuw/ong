package middleware

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/komuw/ong/config"
	"github.com/komuw/ong/internal/tst"
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
		wrappedHandler := csp(echoHandler(msg), domain, config.DefaultCSPPolicy)

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
		wrappedHandler := csp(echoHandler(msg), domain, config.DefaultCSPPolicy)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		attest.Equal(t, rec.Header().Get(cspHeader), config.DefaultCSPPolicy(domain, res.Header.Get(nonceHeader)))
	})

	t.Run("custom policy", func(t *testing.T) {
		t.Parallel()

		domain := "example.com"
		gotDomain := ""
		gotNonce := ""
		policy := func(domain, nonce string) string {
			gotDomain = domain
			gotNonce = nonce
			return fmt.Sprintf("default-src 'none'; script-src 'nonce-%s';", nonce)
		}
		wrappedHandler := csp(echoHandler("hello"), domain, policy)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		nonce := res.Header.Get(nonceHeader)
		attest.Equal(t, gotDomain, domain)
		attest.Equal(t, gotNonce, nonce)
		attest.Equal(t, res.Header.Get(cspHeader), fmt.Sprintf("default-src 'none'; script-src 'nonce-%s';", nonce))
	})

	t.Run("nil policy uses default", func(t *testing.T) {
		t.Parallel()

		domain := "example.com"
		wrappedHandler := csp(echoHandler("hello"), domain, nil)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		attest.Equal(t, res.Header.Get(cspHeader), config.DefaultCSPPolicy(domain, res.Header.Get(nonceHeader)))
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		domain := "example.com"
		// for this concurrency test, we have to re-use the same wrappedHandler
		// so that state is shared and thus we can see if there is any state which is not handled correctly.
		wrappedHandler := csp(echoHandler(msg), domain, config.DefaultCSPPolicy)

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

func TestConfiguredCSPPolicy(t *testing.T) {
	t.Parallel()

	domain := "localhost"
	o := config.CertOpts(domain, tst.SecretKey(), config.DirectIpStrategy, slog.Default(), "", "", nil)
	o.CSPPolicy = func(domain, nonce string) string {
		return fmt.Sprintf("default-src https://%s; script-src 'nonce-%s';", domain, nonce)
	}
	wrappedHandler := All(echoHandler("hello"), o)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "https://localhost/someUri", nil)
	wrappedHandler.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	nonce := res.Header.Get(nonceHeader)
	attest.NotZero(t, nonce)
	attest.Equal(t, res.Header.Get(cspHeader), fmt.Sprintf("default-src https://%s; script-src 'nonce-%s';", domain, nonce))
}

func TestGetCspNonce(t *testing.T) {
	t.Parallel()

	t.Run("can get nonce", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		domain := "example.com"
		wrappedHandler := csp(echoHandler(msg), domain, config.DefaultCSPPolicy)

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
