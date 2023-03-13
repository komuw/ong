package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Most of the code here is inspired by(or taken from):
//   (a) https://github.com/komuw/naz/blob/v0.8.1/naz/ratelimiter.py whose license(MIT) can be found here: https://github.com/komuw/naz/blob/v0.8.1/LICENSE.txt
//   (b) https://github.com/uber-go/ratelimit whose license(MIT) can be found here:                        https://github.com/uber-go/ratelimit/blob/v0.2.0/LICENSE
//

/*
	Github uses a rate limit of 5_000 reqs/hr(1req/sec)
	Twitter uses 900 reqs/15mins(1req/sec)
	Stripe uses 100req/sec.


	- https://docs.github.com/en/developers/apps/building-github-apps/rate-limits-for-github-apps
	- https://developer.twitter.com/en/docs/twitter-api/rate-limits
	- https://stripe.com/docs/rate-limits
*/
// rateLimiterSendRate is the rate limit in requests/sec.
var rateLimiterSendRate = 100.00 //nolint:gochecknoglobals

// rateLimiter is a middleware that limits requests by IP address.
func rateLimiter(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	rl := newRl()
	const retryAfter = 15 * time.Minute

	return func(w http.ResponseWriter, r *http.Request) {
		rl.reSize()

		host := ClientIP(r)
		tb := rl.get(host, rateLimiterSendRate)

		if !tb.allow() {
			err := fmt.Errorf("ong/middleware: rate limited, retry after %s", retryAfter)
			w.Header().Set(ongMiddlewareErrorHeader, err.Error())
			w.Header().Set(retryAfterHeader, fmt.Sprintf("%d", int(retryAfter.Seconds()))) // header should be in seconds(decimal-integer).
			http.Error(
				w,
				err.Error(),
				http.StatusTooManyRequests,
			)
			return
		}

		// todo: maybe also limit max body size using something like `http.MaxBytesHandler`
		// todo: also maybe add another limiter for IP subnet.
		//      see limitation: https://github.com/komuw/ong/issues/17#issuecomment-1114551281

		wrappedHandler(w, r)
	}
}

// rl is a ratelimiter per IP address.
type rl struct {
	mu sync.RWMutex // protects mtb
	// +checklocks:mu
	mtb map[string]*tb
}

func newRl() *rl {
	return &rl{
		mtb: map[string]*tb{},
	}
}

func (r *rl) get(ip string, sendRate float64) *tb {
	r.mu.Lock()
	defer r.mu.Unlock()

	tb, ok := r.mtb[ip]
	if !ok {
		tb = newTb(sendRate)
		r.mtb[ip] = tb
	}

	return tb
}

func (r *rl) reSize() {
	r.mu.RLock()
	_len := len(r.mtb)
	r.mu.RUnlock()

	if _len < 10_000 {
		return
	}

	r.mu.Lock()
	// The size of `tb` is ~56bytes. Although `tb` embeds another struct(mutex), that only has two fileds which are ints.
	// So for 10_000 unique IPs the mem usage is 560KB
	r.mtb = map[string]*tb{}
	r.mu.Unlock()
}

// tb is a simple implementation of the token bucket rate limiting algorithm
// https://en.wikipedia.org/wiki/Token_bucket
type tb struct {
	mu sync.Mutex // protects last and sleepFor
	// +checklocks:mu
	last time.Time
	// +checklocks:mu
	sleepFor time.Duration

	perRequest time.Duration
	maxSlack   time.Duration
}

func newTb(sendRate float64) *tb {
	slack := 10
	per := 1 * time.Second
	perRequest := per / time.Duration(sendRate)
	maxSlack := (-1 * time.Duration(slack) * perRequest)

	return &tb{
		perRequest: perRequest,
		maxSlack:   maxSlack,
	}
}

func (t *tb) allow() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()

	alw := true

	if t.last.IsZero() {
		// this is first request, allow it.
		t.last = now
		return alw
	}

	// sleepFor calculates how much time we should sleep based on
	// the perRequest budget and how long the last request took.
	// Since the request may take longer than the budget, this number
	// can get negative, and is summed across requests.
	t.sleepFor += t.perRequest - now.Sub(t.last)

	// We shouldn't allow sleepFor to get too negative, since it would mean that
	// a service that slowed down a lot for a short period of time would get
	// a much higher RPS following that.
	if t.sleepFor < t.maxSlack {
		t.sleepFor = t.maxSlack
	}

	// If sleepFor is positive, then we should sleep now.
	// fmt.Println("\t t.sleepFor: ", t.sleepFor)
	if t.sleepFor > 0 {
		time.Sleep(t.sleepFor)
		t.last = now.Add(t.sleepFor)
		t.sleepFor = 0
		alw = false
	} else {
		t.last = now
	}

	return alw
}
