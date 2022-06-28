package server

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/akshayjshah/attest"
	"github.com/komuw/goweb/middleware"
)

func someMuxHandler(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, msg)
	}
}

func TestMux(t *testing.T) {
	t.Parallel()

	t.Run("unknown uri", func(t *testing.T) {
		t.Parallel()

		msg := "hello world"
		mux := NewMux(
			Routes{
				NewRoute(
					"/api",
					MethodGet,
					someMuxHandler(msg),
					middleware.WithOpts("localhost"),
				),
			},
		)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/UnknownUri", nil)
		mux.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		attest.Equal(t, res.StatusCode, http.StatusNotFound)
	})

	t.Run("unknown http method", func(t *testing.T) {
		t.Parallel()

		msg := "hello world"
		mux := NewMux(
			Routes{
				NewRoute(
					"/api",
					MethodGet,
					someMuxHandler(msg),
					middleware.WithOpts("localhost"),
				),
			},
		)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodTrace, "/api/", nil)
		mux.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		attest.Equal(t, res.StatusCode, http.StatusMethodNotAllowed)
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		msg := "hello world"
		mux := NewMux(
			Routes{
				NewRoute(
					"/api",
					MethodGet,
					someMuxHandler(msg),
					middleware.WithOpts("localhost"),
				),
			},
		)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/", nil)
		mux.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), msg)
	})
}
