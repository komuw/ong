package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/akshayjshah/attest"
	"golang.org/x/exp/slices"
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

	t.Run("rate limiting happens", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := RateLimiter(someRateLimiterHandler(msg))

		msgsDelivered := []int{}
		start := time.Now().UTC()
		for i := 0; i < 200; i++ {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
			wrappedHandler.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			msgsDelivered = append(msgsDelivered, res.StatusCode)
		}

		rateLimitedreqs := 0
		for _, v := range msgsDelivered {
			if v == http.StatusTooManyRequests {
				rateLimitedreqs = rateLimitedreqs + 1
			}
		}

		timeTakenToDeliver := time.Now().UTC().Sub(start)
		totalMsgsDelivered := len(msgsDelivered)
		effectiveMessageRate := int(float64(totalMsgsDelivered) / timeTakenToDeliver.Seconds())

		attest.True(t, slices.Contains(msgsDelivered, http.StatusTooManyRequests))
		attest.True(t, slices.Contains(msgsDelivered, http.StatusOK))
		attest.True(t, rateLimitedreqs > 10)
		sendRate := 10.00 // gotten from the RateLimiter middleware.
		attest.Approximately(t, effectiveMessageRate, int(sendRate), 4)
	})

	t.Run("bad remoteAddr", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := RateLimiter(someRateLimiterHandler(msg))

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
}

func TestRl(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		{
			sendRate := 14.0
			l := newTb(sendRate)

			msgsDelivered := []int{}
			start := time.Now().UTC()
			for i := 0; i < int(sendRate*4); i++ {
				l.allow()
				msgsDelivered = append(msgsDelivered, 1)
			}
			timeTakenToDeliver := time.Now().UTC().Sub(start)
			totalMsgsDelivered := len(msgsDelivered)
			effectiveMessageRate := int(float64(totalMsgsDelivered) / timeTakenToDeliver.Seconds())

			attest.Approximately(t, effectiveMessageRate, int(sendRate), 3)
		}

		{
			sendRate := 200.0 // 200 requests/second
			l := newTb(sendRate)

			msgsDelivered := []int{}
			start := time.Now().UTC()
			for i := 0; i < int(sendRate*4); i++ {
				l.allow()
				msgsDelivered = append(msgsDelivered, 1)
			}
			timeTakenToDeliver := time.Now().UTC().Sub(start)
			totalMsgsDelivered := len(msgsDelivered)
			effectiveMessageRate := int(float64(totalMsgsDelivered) / timeTakenToDeliver.Seconds())

			attest.Approximately(t, effectiveMessageRate, int(sendRate), 6)
		}
	})
}
