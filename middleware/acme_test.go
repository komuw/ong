package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.akshayshah.org/attest"
)

func someAcmeAppHandler(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, msg)
	}
}

func TestAcme(t *testing.T) {
	t.Parallel()

	t.Run("normal request succeeds", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		domain := "example.com"
		acmeEmail := "hey@example.com"
		acmeDirectoryUrl := "example.net"
		wrappedHandler := acme(someAcmeAppHandler(msg), domain, acmeEmail, acmeDirectoryUrl)
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

	t.Run("acme request succeeds", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		domain := "example.com"
		acmeEmail := "hey@example.com"
		acmeDirectoryUrl := "example.net"
		wrappedHandler := acme(someAcmeAppHandler(msg), domain, acmeEmail, acmeDirectoryUrl)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, acmeURI, nil)
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(
			t,
			res.StatusCode,
			// 404 is the right code in this case because the acme handler
			// expects some token which are not available in test mode.
			// see: https://github.com/golang/crypto/blob/v0.10.0/acme/autocert/autocert.go#L398-L401
			http.StatusNotFound,
		)
		attest.Subsequence(
			t,
			string(rb),
			// This message may change with different versions of `x/crypto/acme/autocert`
			"acme/autocert: certificate cache miss",
		)
	})
}
