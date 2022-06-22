package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/akshayjshah/attest"
)

func someLogHandler(msg string, toErr bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// sleep so that the log middleware has some useful duration metrics to report.
		time.Sleep(3 * time.Millisecond)
		if toErr {
			http.Error(
				w,
				"someLogHandler failed.",
				http.StatusInternalServerError,
			)
			return
		} else {
			fmt.Fprint(w, msg)
			return
		}
	}
}

func TestLogMiddleware(t *testing.T) {

	t.Run("success", func(t *testing.T) {
		msg := "hello"
		wrappedHandler := Log(someLogHandler(msg, false))

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodHead, "/someUri", nil)
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), msg)

		// TODO:
		//   - assert logs.
		//   - assert cookies.
	})

	t.Run("error", func(t *testing.T) {
		msg := "hello"
		wrappedHandler := Log(someLogHandler(msg, true))

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodHead, "/someUri", nil)
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), msg)

		// TODO:
		//   - assert logs.
		//   - assert cookies.
	})
}
