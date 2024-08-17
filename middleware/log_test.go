package middleware

import (
	"bytes"
	"context"
	"fmt"
	"io"
	mathRand "math/rand/v2"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/komuw/ong/id"
	"github.com/komuw/ong/log"

	"go.akshayshah.org/attest"
)

const (
	someLogHandlerHeader = "SomeLogHandlerHeader"
	someLatencyMS        = 3
)

func toLog(t *testing.T, buf *bytes.Buffer) func(w http.ResponseWriter, r http.Request, statusCode int, fields []any) {
	t.Helper()

	const (
		msg = "http_server"
		// rateShedSamplePercent is the percentage of rate limited or loadshed responses that will be logged as errors, by default.
		rateShedSamplePercent = 10
	)

	l := log.New(context.Background(), buf, 500)

	return func(w http.ResponseWriter, r http.Request, statusCode int, fields []any) {
		// Each request should get its own context. That's why we call `log.WithID` for every request.
		reqL := log.WithID(r.Context(), l)

		if (statusCode == http.StatusServiceUnavailable || statusCode == http.StatusTooManyRequests) && w.Header().Get(retryAfterHeader) != "" {
			// We are either in load shedding or rate-limiting.
			// Only log (rateShedSamplePercent)% of the errors.
			shouldLog := mathRand.IntN(100) <= rateShedSamplePercent
			if shouldLog {
				reqL.Error(msg, fields...)
				return
			}
		}

		if statusCode >= http.StatusBadRequest {
			// Both client and server errors.
			if statusCode == http.StatusNotFound || statusCode == http.StatusMethodNotAllowed || statusCode == http.StatusTeapot {
				// These ones are more of an annoyance, than been actual errors.
				reqL.Info(msg, fields...)
				return
			}

			reqL.Error(msg, fields...)
			return
		}

		reqL.Info(msg, fields...)
	}
}

func someLogHandler(successMsg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// sleep so that the log middleware has some useful duration metrics to report.
		time.Sleep(someLatencyMS * time.Millisecond)
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

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		logOutput := &bytes.Buffer{}
		successMsg := "hello"
		wrappedHandler := logger(someLogHandler(successMsg), toLog(t, logOutput), nil)

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
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		logOutput := &bytes.Buffer{}
		errorMsg := "someLogHandler failed"
		successMsg := "hello"
		wrappedHandler := logger(someLogHandler(successMsg), toLog(t, logOutput), nil)

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
			"logID", // should match log.logIDFieldName
			"clientIP",
			"clientFingerPrint",
			"ERROR",
			fmt.Sprintf("%d", someLatencyMS), // latency in millisecond is recorded.
		} {
			attest.Subsequence(t, logOutput.String(), v)
		}
	})

	t.Run("requests share log data.", func(t *testing.T) {
		t.Parallel()

		logOutput := &bytes.Buffer{}
		successMsg := "hello"
		errorMsg := "someLogHandler failed"
		wrappedHandler := logger(someLogHandler(successMsg), toLog(t, logOutput), nil)

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
				"INFO",
				http.MethodHead,
				"FirstUri",
				fmt.Sprint(http.StatusOK),
				// from second request
				http.MethodGet,
				"SecondUri",
				// from third request
				"ERROR",
				http.MethodPost,
				"ThirdUri",
				fmt.Sprint(http.StatusInternalServerError),
				// common
				"durationMS",
			} {
				attest.Subsequence(t, logOutput.String(), v)
			}
		}
	})

	t.Run("re-uses logID", func(t *testing.T) {
		t.Parallel()

		logOutput := &bytes.Buffer{}
		successMsg := "hello"
		wrappedHandler := logger(someLogHandler(successMsg), toLog(t, logOutput), nil)

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
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		logOutput := &bytes.Buffer{}
		successMsg := "hello"
		// for this concurrency test, we have to re-use the same wrappedHandler
		// so that state is shared and thus we can see if there is any state which is not handled correctly.
		wrappedHandler := logger(someLogHandler(successMsg), toLog(t, logOutput), nil)

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
