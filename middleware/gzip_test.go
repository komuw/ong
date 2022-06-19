package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/akshayjshah/attest"
)

func someGzipHandler(msg string, iterations int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		msg = strings.Repeat(msg, iterations)
		fmt.Fprint(w, msg)
	}
}

func TestGzip(t *testing.T) {
	t.Parallel()

	t.Run("http HEAD is not gzipped", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := Gzip(someGzipHandler(msg, 1))

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodHead, "/someUri", nil)
		req.Header.Add(acHeader, "br;q=1.0, gzip;q=0.8, *;q=0.1")
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), msg)
	})

	t.Run("small responses are not gzipped", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		iterations := (defaultMinSize / 100)
		wrappedHandler := Gzip(someGzipHandler(msg, iterations))
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
		attest.Zero(t, res.Header.Get(contentEncoding))
	})

	t.Run("middleware succeds", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		iterations := defaultMinSize * 2
		wrappedHandler := Gzip(someGzipHandler(msg, iterations))
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		req.Header.Add(acHeader, "br;q=1.0, gzip;q=0.8, *;q=0.1")
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		fmt.Println("res.Headers: ", res.Header)
		attest.Equal(t, res.Header.Get(contentEncoding), "gzip")
		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), msg)

	})
}
