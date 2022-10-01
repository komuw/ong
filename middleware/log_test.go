package middleware

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/akshayjshah/attest"
	"github.com/komuw/ong/id"
	"github.com/komuw/ong/log"
)

const someLogHandlerHeader = "SomeLogHandlerHeader"

func someLogHandler(successMsg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// sleep so that the log middleware has some useful duration metrics to report.
		time.Sleep(3 * time.Millisecond)
		if r.Header.Get(someLogHandlerHeader) != "" {
			http.Error(
				w,
				r.Header.Get(someLogHandlerHeader),
				http.StatusInternalServerError,
			)
			return
		} else {
			fmt.Fprint(w, successMsg)
			return
		}
	}
}

func TestLogMiddleware(t *testing.T) {
	t.Parallel()

	getLogger := func(w io.Writer) log.Logger {
		return log.New(w, 500)
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		logOutput := &bytes.Buffer{}
		successMsg := "hello"
		domain := "example.com"
		wrappedHandler := Log(someLogHandler(successMsg), domain, getLogger(logOutput))

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodHead, "/someUri", nil)
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), successMsg)
		attest.Zero(t, logOutput.String())

		logHeader := res.Header.Get(logIDKey)
		attest.NotZero(t, logHeader)
		attest.True(t, len(res.Cookies()) >= 1)
		attest.Equal(t, res.Cookies()[0].Name, logIDKey)
		attest.Equal(t, logHeader, res.Cookies()[0].Value)
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		logOutput := &bytes.Buffer{}
		errorMsg := "someLogHandler failed"
		successMsg := "hello"
		domain := "example.com"
		wrappedHandler := Log(someLogHandler(successMsg), domain, getLogger(logOutput))

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodHead, "/someUri", nil)
		req.Header.Set(someLogHandlerHeader, errorMsg)
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusInternalServerError)
		attest.Equal(t, string(rb), errorMsg+"\n")

		for _, v := range []string{
			"code",
			fmt.Sprint(res.StatusCode),
			"durationMS",
			"logID",
			"requestAddr",
			"error",
		} {
			attest.Subsequence(t, logOutput.String(), v)
		}

		logHeader := res.Header.Get(logIDKey)
		attest.NotZero(t, logHeader)
		attest.True(t, len(res.Cookies()) >= 1)
		attest.Equal(t, res.Cookies()[0].Name, logIDKey)
		attest.Equal(t, logHeader, res.Cookies()[0].Value)
	})

	t.Run("requests share log data.", func(t *testing.T) {
		t.Parallel()

		logOutput := &bytes.Buffer{}
		successMsg := "hello"
		errorMsg := "someLogHandler failed"
		domain := "example.com"
		wrappedHandler := Log(someLogHandler(successMsg), domain, getLogger(logOutput))

		{
			// first request that succeds
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodHead, "/FirstUri", nil)
			wrappedHandler.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)

			attest.Equal(t, res.StatusCode, http.StatusOK)
			attest.Equal(t, string(rb), successMsg)
			attest.Zero(t, logOutput.String())

			logHeader := res.Header.Get(logIDKey)
			attest.NotZero(t, logHeader)
			attest.True(t, len(res.Cookies()) >= 1)
			attest.Equal(t, res.Cookies()[0].Name, logIDKey)
			attest.Equal(t, logHeader, res.Cookies()[0].Value)
		}

		{
			// second request that succeds
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/SecondUri", nil)
			wrappedHandler.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)

			attest.Equal(t, res.StatusCode, http.StatusOK)
			attest.Equal(t, string(rb), successMsg)
			attest.Zero(t, logOutput.String())

			logHeader := res.Header.Get(logIDKey)
			attest.NotZero(t, logHeader)
			attest.True(t, len(res.Cookies()) >= 1)
			attest.Equal(t, res.Cookies()[0].Name, logIDKey)
			attest.Equal(t, logHeader, res.Cookies()[0].Value)
		}

		{
			// third request that errors
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/ThirdUri", nil)
			req.Header.Set(someLogHandlerHeader, errorMsg)
			wrappedHandler.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)

			attest.Equal(t, res.StatusCode, http.StatusInternalServerError)
			attest.Equal(t, string(rb), errorMsg+"\n")

			for _, v := range []string{
				// from first request
				"info",
				http.MethodHead,
				"FirstUri",
				fmt.Sprint(http.StatusOK),
				// from second request
				http.MethodGet,
				"SecondUri",
				// from third request
				"error",
				http.MethodPost,
				"ThirdUri",
				fmt.Sprint(http.StatusInternalServerError),
				// common
				"durationMS",
			} {
				attest.Subsequence(t, logOutput.String(), v)
			}

			logHeader := res.Header.Get(logIDKey)
			attest.NotZero(t, logHeader)
			attest.True(t, len(res.Cookies()) >= 1)
			attest.Equal(t, res.Cookies()[0].Name, logIDKey)
			attest.Equal(t, logHeader, res.Cookies()[0].Value)
		}
	})

	t.Run("re-uses logID", func(t *testing.T) {
		t.Parallel()

		logOutput := &bytes.Buffer{}
		successMsg := "hello"
		domain := "example.com"
		wrappedHandler := Log(someLogHandler(successMsg), domain, getLogger(logOutput))

		someLogID := "hey-some-log-id:" + id.New()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodHead, "/someUri", nil)
		req.AddCookie(&http.Cookie{
			Name:  logIDKey,
			Value: someLogID,
		})
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), successMsg)
		attest.Zero(t, logOutput.String())

		logHeader := res.Header.Get(logIDKey)
		attest.NotZero(t, logHeader)
		attest.True(t, len(res.Cookies()) >= 1)
		attest.Equal(t, res.Cookies()[0].Name, logIDKey)
		attest.Equal(t, logHeader, res.Cookies()[0].Value)
		attest.Equal(t, logHeader, someLogID)
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		logOutput := &bytes.Buffer{}
		successMsg := "hello"
		domain := "example.com"
		// for this concurrency test, we have to re-use the same wrappedHandler
		// so that state is shared and thus we can see if there is any state which is not handled correctly.
		wrappedHandler := Log(someLogHandler(successMsg), domain, getLogger(logOutput))

		runhandler := func() {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
			wrappedHandler.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)

			attest.Equal(t, res.StatusCode, http.StatusOK)
			attest.Equal(t, string(rb), successMsg)
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

func TestGetLogId(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		{
			req := httptest.NewRequest(http.MethodHead, "/someUri", nil)
			id := getLogId(req)
			attest.NotZero(t, id)
		}

		{
			expected := "expected-one"
			req := httptest.NewRequest(http.MethodHead, "/someUri", nil)
			req.Header.Add(logIDKey, expected)
			id := getLogId(req)
			attest.Equal(t, id, expected)
		}

		{
			expected := "expected-two"
			ctx := context.WithValue(context.Background(), log.CtxKey, expected)
			req := httptest.NewRequest(http.MethodHead, "/someUri", nil)
			req = req.WithContext(ctx)
			id := getLogId(req)
			attest.Equal(t, id, expected)
		}

		{
			// header take precedence.
			req := httptest.NewRequest(http.MethodHead, "/someUri", nil)
			req.AddCookie(&http.Cookie{
				Name:  logIDKey,
				Value: "cookie-expected-three",
			})

			expected := "header-logID"
			req.Header.Add(logIDKey, expected)

			req = req.WithContext(
				context.WithValue(context.Background(), log.CtxKey, "context-logID"),
			)

			id := getLogId(req)
			attest.Equal(t, id, expected)
		}
	})
}
