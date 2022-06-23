package middleware

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

		logOutput := &bytes.Buffer{}
		msg := "hello"
		wrappedHandler := Panic(handlerThatPanics(msg, true), logOutput)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		attest.Equal(t, res.StatusCode, http.StatusInternalServerError)

		for _, v := range []string{
			msg,
			fmt.Sprint(http.StatusInternalServerError),
			http.StatusText(http.StatusInternalServerError),
			"logID",
			http.MethodGet,
		} {
			attest.True(t, strings.Contains(logOutput.String(), v))
		}
	})

	t.Run("ok if no panic", func(t *testing.T) {
		t.Parallel()

		logOutput := &bytes.Buffer{}
		msg := "hello"
		wrappedHandler := Panic(handlerThatPanics(msg, false), logOutput)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()
		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Zero(t, logOutput.String())
	})
}
