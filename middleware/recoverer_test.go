package middleware

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/komuw/ong/errors"
	"github.com/komuw/ong/log"

	"go.akshayshah.org/attest"
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

func anotherHandlerThatPanics() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = 90
		someSlice := []string{"zero", "one", "two"}
		_ = "kilo"
		_ = someSlice[16] // panic

		fmt.Fprint(w, "anotherHandlerThatPanics")
	}
}

func TestPanic(t *testing.T) {
	t.Parallel()

	getLogger := func(w io.Writer) *slog.Logger {
		return log.New(context.Background(), w, 500)
	}

	t.Run("ok if no panic", func(t *testing.T) {
		t.Parallel()

		logOutput := &bytes.Buffer{}
		msg := "hello"
		wrappedHandler := recoverer(handlerThatPanics(msg, false, nil), nil, getLogger(logOutput))

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
		wrappedHandler := recoverer(handlerThatPanics(msg, true, nil), nil, getLogger(logOutput))

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
			"logID", // should match log.logIDFieldName
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
		wrappedHandler := recoverer(handlerThatPanics(msg, false, err), nil, getLogger(logOutput))

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
			"logID", // should match log.logIDFieldName
			http.MethodGet,
			"stack",
		} {
			attest.Subsequence(t, logOutput.String(), v, attest.Sprintf("`%s` was not found", v))
		}
		attest.Subsequence(t, logOutput.String(), "stack")
	})

	t.Run("stacktrace has correct line", func(t *testing.T) {
		t.Parallel()

		logOutput := &bytes.Buffer{}
		wrappedHandler := recoverer(anotherHandlerThatPanics(), nil, getLogger(logOutput))

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()
		attest.Equal(t, res.StatusCode, http.StatusInternalServerError)
		attest.Subsequence(t, logOutput.String(), "middleware/recoverer_test.go:41") // line where panic happened.
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		// If &bytes.Buffer{} is not concurrency safe, we can use os.Stderr instead.
		logOutput := &bytes.Buffer{}
		msg := "hey"
		err := errors.New(msg)
		// for this concurrency test, we have to re-use the same wrappedHandler
		// so that state is shared and thus we can see if there is any state which is not handled correctly.
		wrappedHandler := recoverer(handlerThatPanics(msg, false, err), nil, getLogger(logOutput))

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
			wg.Go(func() {
				runhandler()
			})
		}
		wg.Wait()
	})

	t.Run("logger outside recoverer logs panic one time", func(t *testing.T) {
		t.Parallel()

		statusWrittenBeforePanic := func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			panic(errors.New("99 problems after write"))
		}

		tests := []struct {
			name      string
			handler   http.HandlerFunc
			errMsg    string
			wantStack bool
		}{
			{name: "panic with error", handler: handlerThatPanics("hello", false, errors.New("99 problems")), errMsg: "99 problems", wantStack: true},
			{name: "panic with string", handler: handlerThatPanics("hello", true, nil), errMsg: "hello", wantStack: false},
			{name: "panic after status is written", handler: statusWrittenBeforePanic, errMsg: "99 problems after write", wantStack: true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				logOutput := &bytes.Buffer{}
				l := getLogger(logOutput)
				// This is the same order as in allDefaultMiddlewares.
				wrappedHandler := logger(recoverer(tt.handler, nil, l), nil, l)

				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
				wrappedHandler.ServeHTTP(rec, req)

				res := rec.Result()
				defer res.Body.Close()

				logs := strings.TrimSpace(logOutput.String())
				attest.Equal(t, len(strings.Split(logs, "\n")), 1, attest.Sprint(logs))
				attest.Subsequence(t, logs, `"level":"ERROR"`)
				attest.Subsequence(t, logs, `"code":500`)
				attest.Subsequence(t, logs, http.StatusText(http.StatusInternalServerError))
				attest.Subsequence(t, logs, tt.errMsg)
				attest.Subsequence(t, logs, "durationMS") // the line is from logger.
				attest.False(t, strings.Contains(logs, `"code":0`))
				attest.Equal(t, strings.Contains(logs, "stack"), tt.wantStack)
			})
		}
	})
}
