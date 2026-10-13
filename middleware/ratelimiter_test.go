package middleware

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/netip"
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
		start := time.Now()
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
				attest.Equal(t, res.Header.Get(retryAfterHeader), "1")
				attest.NotZero(t, res.Header.Get(ongMiddlewareErrorHeader))
			default:
				t.Fatalf("unexpected status code: %d", res.StatusCode)
			}
			_ = res.Body.Close()
		}

		attest.Equal(t, successfulRequests, int(rateLimit))
		attest.Equal(t, rateLimitedRequests, 1)
		// A rate limited request should not sleep.
		attest.True(t, time.Since(start) < 500*time.Millisecond)
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

// countAllowed returns the number of requests that are allowed, out of n requests from ip at time now.
func countAllowed(r *rl, ip string, now time.Time, n int) int {
	allowed := 0
	for range n {
		if ok, _ := r.allow(ip, now); ok {
			allowed++
		}
	}
	return allowed
}

func TestRl(t *testing.T) {
	t.Parallel()

	const ip = "203.0.113.1"

	t.Run("burst then refuse", func(t *testing.T) {
		t.Parallel()

		for _, rate := range []float64{1, 5, 14, 100, 2.5} {
			r := newRl(rate, maxEntries)
			now := time.Now()
			burst := int(math.Ceil(rate))

			attest.Equal(t, countAllowed(r, ip, now, burst), burst, attest.Sprintf("rate=%v", rate))
			ok, retryAfter := r.allow(ip, now)
			attest.False(t, ok, attest.Sprintf("rate=%v", rate))
			attest.True(t, retryAfter >= 0, attest.Sprintf("rate=%v", rate))
		}
	})

	t.Run("retryAfter", func(t *testing.T) {
		t.Parallel()

		const rate = 5.0
		r := newRl(rate, maxEntries)
		interval := time.Second / rate
		now := time.Now()

		attest.Equal(t, countAllowed(r, ip, now, 5), 5)

		ok, retryAfter := r.allow(ip, now)
		attest.False(t, ok)
		attest.Equal(t, retryAfter, interval)

		// A short time later, the request is still refused.
		now = now.Add(interval / 2)
		ok, retryAfter = r.allow(ip, now)
		attest.False(t, ok)
		attest.Equal(t, retryAfter, interval/2)

		// After retryAfter, exactly one request is allowed.
		now = now.Add(retryAfter)
		attest.Equal(t, countAllowed(r, ip, now, 2), 1)
	})

	t.Run("requests spread over time are not refused", func(t *testing.T) {
		t.Parallel()

		// This was a bug in the previous token bucket implementation; every (rate+1)th request was refused.
		const rate = 5.0
		r := newRl(rate, maxEntries)
		now := time.Now()
		for i := range 100 {
			ok, _ := r.allow(ip, now)
			attest.True(t, ok, attest.Sprintf("i=%d", i))
			now = now.Add(1 * time.Hour)
		}
	})

	t.Run("steady rate", func(t *testing.T) {
		t.Parallel()

		for _, rate := range []float64{1, 14, 100, 200} {
			r := newRl(rate, maxEntries)
			now := time.Now()
			const duration = 10 * time.Second
			const sendRate = 1_000 // client sends 1000 req/sec.

			allowed := 0
			for range int(duration.Seconds()) * sendRate {
				if ok, _ := r.allow(ip, now); ok {
					allowed++
				}
				now = now.Add(time.Second / sendRate)
			}

			want := int(rate*duration.Seconds()) + int(math.Ceil(rate)) // rate plus initial burst.
			attest.Approximately(t, allowed, want, 2, attest.Sprintf("rate=%v", rate))
		}
	})

	t.Run("rate below one uses default", func(t *testing.T) {
		t.Parallel()

		for _, rate := range []float64{0, 0.5, -3} {
			r := newRl(rate, maxEntries)
			now := time.Now()
			burst := int(config.DefaultRateLimit)
			attest.Equal(t, countAllowed(r, ip, now, burst+1), burst, attest.Sprintf("rate=%v", rate))
		}
	})

	t.Run("very high rate", func(t *testing.T) {
		t.Parallel()

		for _, rate := range []float64{2e7, 1e9, 1e12, math.Inf(1)} {
			r := newRl(rate, maxEntries)
			attest.Equal(t, countAllowed(r, ip, time.Now(), 1_000), 1_000, attest.Sprintf("rate=%v", rate))
			attest.Equal(t, countAllowed(r, "2001:db8::1", time.Now(), 1_000), 1_000, attest.Sprintf("rate=%v", rate))

			for _, tiers := range [][]tier{r.ipv4Tiers, r.ipv6Tiers, r.otherTiers} {
				for _, tr := range tiers {
					attest.True(t, tr.interval >= time.Nanosecond, attest.Sprintf("rate=%v, bits=%d", rate, tr.bits))
				}
			}
		}
	})

	t.Run("ipv4 addresses are independent", func(t *testing.T) {
		t.Parallel()

		r := newRl(2, maxEntries)
		now := time.Now()
		attest.Equal(t, countAllowed(r, "203.0.113.1", now, 3), 2)
		attest.Equal(t, countAllowed(r, "203.0.113.2", now, 3), 2)
	})

	t.Run("ipv6 same /64 shares a limit", func(t *testing.T) {
		t.Parallel()

		r := newRl(2, maxEntries)
		now := time.Now()
		attest.Equal(t, countAllowed(r, "2001:db8:0:1::1", now, 1), 1)
		attest.Equal(t, countAllowed(r, "2001:db8:0:1::2", now, 1), 1)
		attest.Equal(t, countAllowed(r, "2001:db8:0:1:ffff::3", now, 1), 0)
		// A different /64.
		attest.Equal(t, countAllowed(r, "2001:db8:0:2::1", now, 3), 2)
	})

	t.Run("ipv6 /56 and /48 tiers", func(t *testing.T) {
		t.Parallel()

		const rate = 2
		r := newRl(rate, maxEntries)
		now := time.Now()

		// Each /64 in one /56 sends many requests. The /56 tier allows at most 8*burst.
		allowed := 0
		for i := range 256 {
			allowed += countAllowed(r, fmt.Sprintf("2001:db8:0:%x::1", i), now, 10)
		}
		attest.Equal(t, allowed, rate*ipv6Prefix56Multiplier)

		// Each /56 in one /48 sends many requests. The /48 tier allows at most 64*burst; the /56 above already used some.
		allowed = 0
		for i := 1; i < 256; i++ {
			for j := range 16 {
				allowed += countAllowed(r, fmt.Sprintf("2001:db8:0:%x%02x::1", i, j), now, 10)
			}
		}
		attest.Equal(t, allowed, rate*ipv6Prefix48Multiplier-rate*ipv6Prefix56Multiplier)

		// A different /48 is not affected.
		attest.Equal(t, countAllowed(r, "2001:db9::1", now, 3), rate)
	})

	t.Run("refusal does not change state", func(t *testing.T) {
		t.Parallel()

		r := newRl(1, maxEntries)
		now := time.Now()
		// Fill the /56 tier using different /64s.
		for i := range ipv6Prefix56Multiplier {
			attest.Equal(t, countAllowed(r, fmt.Sprintf("2001:db8:0:%x::1", i), now, 1), 1)
		}

		key64 := netip.MustParsePrefix("2001:db8:0:ff::/64")
		key48 := netip.MustParsePrefix("2001:db8::/48")
		r.mu.Lock()
		before48 := r.tats[key48]
		r.mu.Unlock()

		// The /64 tier allows this, but the /56 tier refuses it.
		ok, _ := r.allow("2001:db8:0:ff::1", now)
		attest.False(t, ok)

		r.mu.Lock()
		_, has64 := r.tats[key64]
		after48 := r.tats[key48]
		r.mu.Unlock()
		attest.False(t, has64)
		attest.True(t, before48.Equal(after48))
	})

	t.Run("address normalisation", func(t *testing.T) {
		t.Parallel()

		r := newRl(2, maxEntries)
		now := time.Now()
		// IPv4-mapped IPv6 is the same as IPv4.
		attest.Equal(t, countAllowed(r, "::ffff:203.0.113.1", now, 1), 1)
		attest.Equal(t, countAllowed(r, "203.0.113.1", now, 2), 1)

		// Zones are dropped.
		attest.Equal(t, countAllowed(r, "fe80::1%eth0", now, 1), 1)
		attest.Equal(t, countAllowed(r, "fe80::1%eth1", now, 2), 1)

		// Empty and invalid addresses share one key.
		attest.Equal(t, countAllowed(r, "", now, 1), 1)
		attest.Equal(t, countAllowed(r, "BadRemoteAddr", now, 2), 1)
	})

	t.Run("sweep removes expired entries", func(t *testing.T) {
		t.Parallel()

		r := newRl(1, maxEntries)
		now := time.Now()
		attest.Equal(t, countAllowed(r, "203.0.113.1", now, 1), 1)
		attest.Equal(t, countAllowed(r, "203.0.113.2", now.Add(sweepPeriod), 1), 1)

		// At this time, the first entry has expired and the second one has not.
		now = now.Add(sweepPeriod + 500*time.Millisecond)
		r.mu.Lock()
		r.lastSweep = time.Time{}
		r.sweep(now)
		_, has1 := r.tats[netip.MustParsePrefix("203.0.113.1/32")]
		_, has2 := r.tats[netip.MustParsePrefix("203.0.113.2/32")]
		r.mu.Unlock()
		attest.False(t, has1)
		attest.True(t, has2)

		// The active limit survives the sweep.
		attest.Equal(t, countAllowed(r, "203.0.113.2", now, 1), 0)
	})

	t.Run("hard cap", func(t *testing.T) {
		t.Parallel()

		const capacity = 100
		r := newRl(1, capacity)
		now := time.Now()
		for i := range 1_000 {
			attest.Equal(t, countAllowed(r, fmt.Sprintf("2001:db8:%x::1", i), now, 1), 1)

			r.mu.Lock()
			l := len(r.tats)
			r.mu.Unlock()
			attest.True(t, l <= capacity+len(r.ipv6Tiers), attest.Sprintf("len=%d", l))
		}
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		r := newRl(10, 50)
		wg := &sync.WaitGroup{}
		for i := range 20 {
			wg.Go(func() {
				for j := range 100 {
					r.allow(fmt.Sprintf("2001:db8:%x::%x", i, j), time.Now())
				}
			})
		}
		wg.Wait()
	})
}

func BenchmarkRateLimiter(b *testing.B) {
	wrappedHandler := rateLimiter(someRateLimiterHandler("hello"), 1e9)
	ips := []string{"203.0.113.1:80", "[2001:db8::1]:80"}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
			req.RemoteAddr = ips[i%len(ips)]
			wrappedHandler.ServeHTTP(rec, req)
			i++
		}
	})
}
