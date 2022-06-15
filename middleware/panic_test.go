package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/akshayjshah/attest"
)

func handlerThatPanics(msg string, shouldPanic bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		x := 3 + 9
		_ = x
		if shouldPanic {
			panic(msg)
		}
		fmt.Fprint(w, msg)
	}
}

func TestPanic(t *testing.T) {
	t.Parallel()

	t.Run("catches panics", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := Panic(handlerThatPanics(msg, true))

		rec := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		wrappedHandler.ServeHTTP(rec, r)

		res := rec.Result()
		defer res.Body.Close()

		attest.Equal(t, res.StatusCode, http.StatusInternalServerError)
	})

	t.Run("ok if no panic", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := Panic(handlerThatPanics(msg, false))

		rec := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		wrappedHandler.ServeHTTP(rec, r)

		res := rec.Result()
		defer res.Body.Close()
		attest.Equal(t, res.StatusCode, http.StatusOK)
	})
}
