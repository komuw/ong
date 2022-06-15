package middleware

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/akshayjshah/attest"
)

const nonceHeader = "CUSTOM-CSP-NONCE-TEST-HEADER"

// echoHandler echos back in the response, the msg that was passed in.
func echoHandler(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(nonceHeader, GetCspNonce(r.Context()))
		fmt.Fprint(w, msg)
	}
}

func TestSecurity(t *testing.T) {
	t.Parallel()

	t.Run("middleware succeds", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		domain := "example.com"
		wrappedHandler := Security(echoHandler(msg), domain)

		rec := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		wrappedHandler.ServeHTTP(rec, r)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), msg)
	})

	t.Run("all headers set succsfully", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		domain := "example.com"
		wrappedHandler := Security(echoHandler(msg), domain)

		rec := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		r.TLS = &tls.ConnectionState{} // fake tls so that the STS header is set.
		wrappedHandler.ServeHTTP(rec, r)

		res := rec.Result()
		defer res.Body.Close()

		expect := map[string]string{
			permissionsPolicyHeader: "interest-cohort=()",
			cspHeader:               getCsp(domain, res.Header.Get(nonceHeader)),
			xContentOptionsHeader:   "nosniff",
			xFrameHeader:            "DENY",
			corpHeader:              "same-site",
			coopHeader:              "same-origin",
			referrerHeader:          "strict-origin-when-cross-origin",
			stsHeader:               getSts(15 * 24 * time.Hour),
		}

		for k, v := range expect {
			got := rec.Header().Get(k)
			attest.Equal(t, got, v)
		}
	})
}

func TestGetCspNonce(t *testing.T) {
	t.Parallel()

	t.Run("can get nonce", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		domain := "example.com"
		wrappedHandler := Security(echoHandler(msg), domain)

		rec := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		wrappedHandler.ServeHTTP(rec, r)

		res := rec.Result()
		defer res.Body.Close()

		got := res.Header.Get(nonceHeader)
		attest.NotZero(t, got)
		attest.True(t, got != defaultNonce)
	})
}
