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

func someLogHandler(successMsg string, errorMsg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// sleep so that the log middleware has some useful duration metrics to report.
		time.Sleep(3 * time.Millisecond)
		if errorMsg != "" {
			http.Error(
				w,
				errorMsg,
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

		successMsg := "hello"
		wrappedHandler := Log(someLogHandler(successMsg, ""))

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodHead, "/someUri", nil)
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), successMsg)

		// TODO:
		//   - assert logs.
		//   - assert cookies.
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		errorMsg := "someLogHandler failed"
		successMsg := "hello"
		wrappedHandler := Log(someLogHandler(successMsg, errorMsg))

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodHead, "/someUri", nil)
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusInternalServerError)
		attest.Equal(t, string(rb), errorMsg+"\n")

		// TODO:
		//   - assert logs.
		//   - assert cookies.
	})

	t.Run("requests share log data.", func(t *testing.T) {
		t.Parallel()

		successMsg := "hello"
		wrappedHandler := Log(someLogHandler(successMsg, ""))

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

			fmt.Println("came here.")
		}

		{
			// third request that errors
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/ThirdUri", nil)
			wrappedHandler.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)

			attest.Equal(t, res.StatusCode, http.StatusOK)
			attest.Equal(t, string(rb), successMsg)

			fmt.Println("came here.")
		}

		// TODO:
		//   - that after first request we DO NOT log
		//   - that after second request  we DO NOT log
		//   - that after third request.
		//        - we log the first request had http HEAD and `FirstUri`
		//        - we log the second request had http GET and `SecondUri`
		//        - we log the third request had http POST and `ThirdUri`

		// TODO:
		//   - assert logs.
		//   - assert cookies.

	})
}
