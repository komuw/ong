package middleware

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/komuw/ong/id"
	"github.com/komuw/ong/internal/octx"

	"go.akshayshah.org/attest"
)

const someTraceHandlerHeader = "someTraceHandlerHeader"

func someTraceHandler(successMsg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// sleep so that the trace middleware has some useful duration metrics to report.
		time.Sleep(3 * time.Millisecond)
		if r.Header.Get(someTraceHandlerHeader) != "" {
			http.Error(
				w,
				r.Header.Get(someTraceHandlerHeader),
				http.StatusInternalServerError,
			)
			return
		} else {
			fmt.Fprint(w, successMsg)
			return
		}
	}
}

func TestTraceMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		successMsg := "hello"
		domain := "example.com"
		wrappedHandler := trace(someTraceHandler(successMsg), domain)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodHead, "/someUri", nil)
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), successMsg)

		logHeader := res.Header.Get(logIDKey)
		attest.NotZero(t, logHeader)
		attest.True(t, len(res.Cookies()) >= 1)
		attest.Equal(t, res.Cookies()[0].Name, logIDKey)
		attest.Equal(t, logHeader, res.Cookies()[0].Value)
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		errorMsg := "someTraceHandler failed"
		successMsg := "hello"
		domain := "example.com"
		wrappedHandler := trace(someTraceHandler(successMsg), domain)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodHead, "/someUri", nil)
		req.Header.Set(someTraceHandlerHeader, errorMsg)
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusInternalServerError)
		attest.Equal(t, string(rb), errorMsg+"\n")

		logHeader := res.Header.Get(logIDKey)
		attest.NotZero(t, logHeader)
		attest.True(t, len(res.Cookies()) >= 1)
		attest.Equal(t, res.Cookies()[0].Name, logIDKey)
		attest.Equal(t, logHeader, res.Cookies()[0].Value)
	})

	t.Run("requests share data.", func(t *testing.T) {
		t.Parallel()

		successMsg := "hello"
		errorMsg := "someTraceHandler failed"
		domain := "example.com"
		wrappedHandler := trace(someTraceHandler(successMsg), domain)

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
			req.Header.Set(someTraceHandlerHeader, errorMsg)
			wrappedHandler.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)

			attest.Equal(t, res.StatusCode, http.StatusInternalServerError)
			attest.Equal(t, string(rb), errorMsg+"\n")

			logHeader := res.Header.Get(logIDKey)
			attest.NotZero(t, logHeader)
			attest.True(t, len(res.Cookies()) >= 1)
			attest.Equal(t, res.Cookies()[0].Name, logIDKey)
			attest.Equal(t, logHeader, res.Cookies()[0].Value)
		}
	})

	t.Run("re-uses logID", func(t *testing.T) {
		t.Parallel()

		successMsg := "hello"
		domain := "example.com"
		wrappedHandler := trace(someTraceHandler(successMsg), domain)

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

		logHeader := res.Header.Get(logIDKey)
		attest.NotZero(t, logHeader)
		attest.True(t, len(res.Cookies()) >= 1)
		attest.Equal(t, res.Cookies()[0].Name, logIDKey)
		attest.Equal(t, logHeader, res.Cookies()[0].Value)
		attest.Equal(t, logHeader, someLogID)
	})
	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		successMsg := "hello"
		domain := "example.com"
		// for this concurrency test, we have to re-use the same wrappedHandler
		// so that state is shared and thus we can see if there is any state which is not handled correctly.
		wrappedHandler := trace(someTraceHandler(successMsg), domain)

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
			ctx := context.WithValue(context.Background(), octx.LogCtxKey, expected)
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
				context.WithValue(context.Background(), octx.LogCtxKey, "context-logID"),
			)

			id := getLogId(req)
			attest.Equal(t, id, expected)
		}
	})
}
