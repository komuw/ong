package middleware

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/akshayjshah/attest"
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

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		logOutput := &bytes.Buffer{}
		successMsg := "hello"
		domain := "example.com"
		wrappedHandler := Log(someLogHandler(successMsg), logOutput, domain)

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
		wrappedHandler := Log(someLogHandler(successMsg), logOutput, domain)

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
			"bytes",
			"code",
			fmt.Sprint(res.StatusCode),
			"durationMS",
			"logID",
			"requestAddr",
			"error",
		} {
			attest.True(t, strings.Contains(logOutput.String(), v))
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
		wrappedHandler := Log(someLogHandler(successMsg), logOutput, domain)

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
				"bytes",
			} {
				attest.True(t, strings.Contains(logOutput.String(), v))
			}

			logHeader := res.Header.Get(logIDKey)
			attest.NotZero(t, logHeader)
			attest.True(t, len(res.Cookies()) >= 1)
			attest.Equal(t, res.Cookies()[0].Name, logIDKey)
			attest.Equal(t, logHeader, res.Cookies()[0].Value)
		}
	})
}
