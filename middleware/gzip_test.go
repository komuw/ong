package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/akshayjshah/attest"
)

func someGzipHandler(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, msg)
	}
}

func TestGzip(t *testing.T) {
	t.Parallel()

	t.Run("middleware succeds", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := Gzip(someGzipHandler(msg))

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		req.Header.Add(acHeader, "br;q=1.0, gzip;q=0.8, *;q=0.1")
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), msg)

		fmt.Println("res.Headers: ", res.Header)
	})
}
