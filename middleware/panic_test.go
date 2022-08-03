package middleware

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/komuw/ong/errors"
	"github.com/komuw/ong/log"

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

	getLogger := func(w io.Writer) log.Logger {
		return log.New(context.Background(), w, 500)
	}

	t.Run("ok if no panic", func(t *testing.T) {
		t.Parallel()

		logOutput := &bytes.Buffer{}
		msg := "hello"
		wrappedHandler := Panic(handlerThatPanics(msg, false, nil), getLogger(logOutput))

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
		wrappedHandler := Panic(handlerThatPanics(msg, true, nil), getLogger(logOutput))

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
			attest.Subsequence(t, logOutput.String(), v)
		}
		attest.False(t, strings.Contains(logOutput.String(), "stack"))
	})

	t.Run("panics with err have stacktrace", func(t *testing.T) {
		t.Parallel()

		logOutput := &bytes.Buffer{}
		msg := "hello"
		errMsg := "99 problems"
		err := errors.New(errMsg)
		wrappedHandler := Panic(handlerThatPanics(msg, false, err), getLogger(logOutput))

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
			attest.Subsequence(t, logOutput.String(), v, attest.Sprintf("`%s` was not found", v))
		}
		attest.Subsequence(t, logOutput.String(), "stack")
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		// &bytes.Buffer{} is not concurrency safe, so we use os.Stderr instead.
		logOutput := os.Stderr
		msg := "hey"
		err := errors.New(msg)
		// for this concurrency test, we have to re-use the same wrappedHandler
		// so that state is shared and thus we can see if there is any state which is not handled correctly.
		wrappedHandler := Panic(handlerThatPanics(msg, false, err), getLogger(logOutput))

		runhandler := func() {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
			wrappedHandler.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			attest.Equal(t, res.StatusCode, http.StatusInternalServerError)
		}

		wg := &sync.WaitGroup{}
		for rN := 0; rN <= 10; rN++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				runhandler()
			}()
		}
		wg.Wait()
	})
}
