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

// TODO: rename this to loadShedder?
func someRateLimiterHandler(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		fmt.Fprint(w, msg)
	}
}

// TODO: rename this to loadShedder?
func TestRateLimiter(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := RateLimiter(someRateLimiterHandler(msg))

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), msg)
	})

	// t.Run("success", func(t *testing.T) {
	// 	t.Parallel()

	// 	msg := "hello"
	// 	wrappedHandler := RateLimiter(someRateLimiterHandler(msg))

	// 	rec := httptest.NewRecorder()
	// 	req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
	// 	wrappedHandler.ServeHTTP(rec, req)

	// 	res := rec.Result()
	// 	defer res.Body.Close()

	// 	rb, err := io.ReadAll(res.Body)
	// 	attest.Ok(t, err)

	// 	attest.Equal(t, res.StatusCode, http.StatusOK)
	// 	attest.Equal(t, string(rb), msg)
	// })

	// t.Run("bad remoteAddr", func(t *testing.T) {
	// 	t.Parallel()

	// 	msg := "hello"
	// 	wrappedHandler := RateLimiter(someRateLimiterHandler(msg))

	// 	rec := httptest.NewRecorder()
	// 	req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
	// 	req.RemoteAddr = "BadRemoteAddr"
	// 	wrappedHandler.ServeHTTP(rec, req)

	// 	res := rec.Result()
	// 	defer res.Body.Close()

	// 	rb, err := io.ReadAll(res.Body)
	// 	attest.Ok(t, err)

	// 	attest.Equal(t, res.StatusCode, http.StatusOK)
	// 	attest.Equal(t, string(rb), msg)
	// })
}

func TestPercentile(t *testing.T) {
	t.Run("todo", func(t *testing.T) {
		lq := latencyQueue{
			latency{duration: 5 * time.Second},
			latency{duration: 6 * time.Second},
			latency{duration: 7 * time.Second},
			latency{duration: 8 * time.Second},
			latency{duration: 9 * time.Second},
			latency{duration: 0 * time.Second},
			latency{duration: 1 * time.Second},
			latency{duration: 2 * time.Second},
			latency{duration: 3 * time.Second},
			latency{duration: 4 * time.Second},
		}
		got := percentile(lq, 0.25)
		attest.Equal(t, got, 2*time.Second)
	})
}
