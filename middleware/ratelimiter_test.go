package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/komuw/ong/config"
	"go.akshayshah.org/attest"
)

func someRateLimiterHandler(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, msg)
	}
}

func TestRateLimiter(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := rateLimiter(someRateLimiterHandler(msg), config.DefaultRateLimit)

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

	t.Run("rate limiting happens", func(t *testing.T) {
		t.Parallel()

		const rateLimit = 5.0
		msg := "hello"
		wrappedHandler := rateLimiter(someRateLimiterHandler(msg), rateLimit)

		successfulRequests := 0
		rateLimitedRequests := 0
		for range int(rateLimit) + 1 {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
			wrappedHandler.ServeHTTP(rec, req)

			res := rec.Result()
			switch res.StatusCode {
			case http.StatusOK:
				successfulRequests++
			case http.StatusTooManyRequests:
				rateLimitedRequests++
				attest.Equal(t, res.Header.Get(retryAfterHeader), "900")
				attest.NotZero(t, res.Header.Get(ongMiddlewareErrorHeader))
			default:
				t.Fatalf("unexpected status code: %d", res.StatusCode)
			}
			_ = res.Body.Close()
		}

		attest.Equal(t, successfulRequests, int(rateLimit))
		attest.Equal(t, rateLimitedRequests, 1)
	})

	t.Run("bad remoteAddr", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := rateLimiter(someRateLimiterHandler(msg), config.DefaultRateLimit)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		req.RemoteAddr = "BadRemoteAddr"
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), msg)
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		// for this concurrency test, we have to re-use the same wrappedHandler
		// so that state is shared and thus we can see if there is any state which is not handled correctly.
		wrappedHandler := rateLimiter(someRateLimiterHandler(msg), config.DefaultRateLimit)

		runhandler := func() {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
			wrappedHandler.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)

			attest.Equal(t, res.StatusCode, http.StatusOK)
			attest.Equal(t, string(rb), msg)
		}

		wg := &sync.WaitGroup{}
		for rN := 0; rN <= 14; rN++ {
			wg.Go(func() {
				runhandler()
			})
		}
		wg.Wait()
	})
}

func TestRl(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		{
			sendRate := 14.0
			l := newTb(sendRate)

			msgsDelivered := 0
			start := time.Now().UTC()
			for i := 0; i < int(sendRate*4); i++ {
				if l.allow() {
					msgsDelivered = msgsDelivered + 1
				}
			}
			timeTakenToDeliver := time.Now().UTC().Sub(start)
			effectiveMessageRate := int(float64(msgsDelivered) / timeTakenToDeliver.Seconds())

			attest.Approximately(t, effectiveMessageRate, int(sendRate), 4)
		}

		{
			sendRate := 200.0 // 200 requests/second
			l := newTb(sendRate)

			msgsDelivered := 0
			start := time.Now().UTC()
			for i := 0; i < int(sendRate*4); i++ {
				if l.allow() {
					msgsDelivered = msgsDelivered + 1
				}
			}
			timeTakenToDeliver := time.Now().UTC().Sub(start)
			effectiveMessageRate := int(float64(msgsDelivered) / timeTakenToDeliver.Seconds())

			attest.Approximately(t, effectiveMessageRate, int(sendRate), 6)
		}
	})
}
