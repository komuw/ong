package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/akshayjshah/attest"
)

const nonceHeader = "CUSTOM-CSP-NONCE"

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
		host := "example.com"
		wrappedHandler := Security(echoHandler(msg), host)

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
		host := "example.com"
		wrappedHandler := Security(echoHandler(msg), host)

		rec := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		wrappedHandler.ServeHTTP(rec, r)

		res := rec.Result()
		defer res.Body.Close()

		expect := map[string]string{
			permissionsPolicyHeader: "interest-cohort=()",
			cspHeader:               getCsp(host, res.Header.Get(nonceHeader)),
			xContentOptionsHeader:   "nosniff",
			xFrameHeader:            "DENY",
			corpHeader:              "same-site",
			coopHeader:              "same-origin",
			referrerHeader:          "strict-origin-when-cross-origin",
			// stsHeader:             "Strict-Transport-Security",
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
		host := "example.com"
		wrappedHandler := Security(echoHandler(msg), host)

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
