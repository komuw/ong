package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
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
		wrappedHandler := rateLimiter(someRateLimiterHandler(msg))

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
		wrappedHandler := rateLimiter(someRateLimiterHandler(msg))

		msgsDelivered := []int{}
		start := time.Now().UTC()
		for i := 0; i < int(rateLimiterSendRate*6); i++ {
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
		attest.True(t, rateLimitedreqs > 4)
		attest.Approximately(t, effectiveMessageRate, int(rateLimiterSendRate), 4)
	})

	t.Run("bad remoteAddr", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := rateLimiter(someRateLimiterHandler(msg))

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
		wrappedHandler := rateLimiter(someRateLimiterHandler(msg))

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
			wg.Add(1)
			go func() {
				defer wg.Done()
				runhandler()
			}()
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
				l.allow()
				msgsDelivered = msgsDelivered + 1
			}
			timeTakenToDeliver := time.Now().UTC().Sub(start)
			effectiveMessageRate := int(float64(msgsDelivered) / timeTakenToDeliver.Seconds())

			attest.Equal(t, effectiveMessageRate, int(sendRate))
		}

		{
			sendRate := 200.0 // 200 requests/second
			l := newTb(sendRate)

			msgsDelivered := 0
			start := time.Now().UTC()
			for i := 0; i < int(sendRate*4); i++ {
				l.allow()
				msgsDelivered = msgsDelivered + 1
			}
			timeTakenToDeliver := time.Now().UTC().Sub(start)
			effectiveMessageRate := int(float64(msgsDelivered) / timeTakenToDeliver.Seconds())

			attest.Equal(t, effectiveMessageRate, int(sendRate))
		}
	})
}

var resultBenchmarkRl bool

func BenchmarkRl(b *testing.B) {
	var r bool

	sendRate := 23.0
	l := newTb(sendRate)

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < int(sendRate)*b.N; n++ {
		r = l.allow()
	}

	// always store the result to a package level variable
	// so the compiler cannot eliminate the Benchmark itself.
	resultBenchmarkRl = r
}
