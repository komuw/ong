package middleware

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/komuw/goweb/errors"

	"github.com/akshayjshah/attest"
)

func handlerThatPanics(msg string, shouldPanic bool, err error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		x := 3 + 9
		_ = x
		if shouldPanic {
			panic(msg)
		}
		if err != nil {
			panic(err)
		}

		fmt.Fprint(w, msg)
	}
}

func TestPanic(t *testing.T) {
	t.Parallel()

	t.Run("ok if no panic", func(t *testing.T) {
		t.Parallel()

		logOutput := &bytes.Buffer{}
		msg := "hello"
		wrappedHandler := Panic(handlerThatPanics(msg, false, nil), logOutput)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()
		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Zero(t, logOutput.String())
	})

	t.Run("catches panics", func(t *testing.T) {
		t.Parallel()

		logOutput := &bytes.Buffer{}
		msg := "hello"
		wrappedHandler := Panic(handlerThatPanics(msg, true, nil), logOutput)

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
		attest.False(t, strings.Contains(logOutput.String(), "stack"))
	})

	t.Run("panics with err have stacktrace", func(t *testing.T) {
		t.Parallel()

		logOutput := &bytes.Buffer{}
		msg := "hello"
		errMsg := "99 problems"
		err := errors.New(errMsg)
		wrappedHandler := Panic(handlerThatPanics(msg, false, err), logOutput)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		attest.Equal(t, res.StatusCode, http.StatusInternalServerError)

		for _, v := range []string{
			errMsg,
			fmt.Sprint(http.StatusInternalServerError),
			http.StatusText(http.StatusInternalServerError),
			"logID",
			http.MethodGet,
			"stack",
		} {
			attest.True(t, strings.Contains(logOutput.String(), v), attest.Sprintf("`%s` was not found", v))
		}
		attest.True(t, strings.Contains(logOutput.String(), "stack"))
	})
}
