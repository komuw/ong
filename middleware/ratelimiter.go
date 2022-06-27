package middleware

import (
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Most of the code here is insipired by(or taken from):
//   (a) https://github.com/komuw/naz/blob/v0.8.1/naz/ratelimiter.py whose license(MIT) can be found here: https://github.com/komuw/naz/blob/v0.8.1/LICENSE.txt

const (
	/*
		Github uses a rate limit of 5_000 reqs/hr(1req/sec)
		Twitter uses 900 reqs/15mins(1req/sec)
		Stripe uses 100req/sec.


		- https://docs.github.com/en/developers/apps/building-github-apps/rate-limits-for-github-apps
		- https://developer.twitter.com/en/docs/twitter-api/rate-limits
		- https://stripe.com/docs/rate-limits
	*/
	rateLimiterSendRate = 100.00 // requests/sec
)

// RateLimiter is a middleware that limits requests by IP address.
func RateLimiter(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	rl := newRl()
	retryAfter := 15 * time.Minute
	reqs := 0

	return func(w http.ResponseWriter, r *http.Request) {
		reqs := reqs + 1
		if reqs > 5_000 {
			// The size of `tb` is ~56bytes. Although `tb` embeds another struct(mutex),
			// that only has two fileds which are ints. So for 5_000 requests the mem usage is 280KB
			rl.reSize()
		}

		tb := rl.get(fetchIP(r.RemoteAddr), rateLimiterSendRate)

		if !tb.allow() {
			err := fmt.Errorf("rate limited, retry after %s", retryAfter)
			w.Header().Set(gowebMiddlewareErrorHeader, err.Error())
			w.Header().Set(retryAfterHeader, fmt.Sprintf("%d", int(retryAfter.Seconds()))) // header should be in seconds(decimal-integer).
			http.Error(
				w,
				err.Error(),
				http.StatusTooManyRequests,
			)
			return
		}

		wrappedHandler(w, r)
	}
}

func fetchIP(remoteAddr string) string {
	// the documentation of `http.Request.RemoteAddr` says:
	// RemoteAddr is not filled in by ReadRequest and has no defined format.
	// So we cant rely on it been present, or having a given format.
	// Although, net/http makes a good effort of availing it & in a standard format.
	//
	ipAddr := strings.Split(remoteAddr, ":")
	return ipAddr[0]
}

// rl is a ratelimiter per IP address.
type rl struct {
	mu  sync.Mutex // protects mtb
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
	r.mu.Lock()
	r.mtb = map[string]*tb{}
	r.mu.Unlock()
}

// tb is a simple implementation of the token bucket rate limiting algorithm
// https://en.wikipedia.org/wiki/Token_bucket
type tb struct {
	mu sync.Mutex // protects all the other fields.

	sendRate       float64 // In req/seconds
	maxTokens      float64
	tokens         float64
	delayForTokens time.Duration
	// updatedAt is the time at which this operation took place.
	// We could have ideally used a `time.Time` as its type; but we wanted the latency struct to be minimal in size.
	updatedAt         int64
	messagesDelivered float64
}

func newTb(sendRate float64) *tb {
	return &tb{
		sendRate:  sendRate,
		maxTokens: sendRate,
		tokens:    sendRate,
		// if sendRate is in req/Sec, delayForTokens ought to be in seconds.
		// if sendRate is in req/ms, delayForTokens ought to be in ms.
		delayForTokens:    1 * time.Second,
		updatedAt:         time.Now().UTC().Unix(),
		messagesDelivered: 0,
	}
}

func (t *tb) allow() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	alw := true
	if t.tokens < 1 {
		alw = false
	}

	t.limit()
	return alw
}

// limit is a private api(thus needs no locking). It should only ever be called by `tb.allow`
func (t *tb) limit() {
	for t.tokens < 1 {
		t.addNewTokens()
		time.Sleep(t.delayForTokens)
	}
	t.messagesDelivered = t.messagesDelivered + 1
	t.tokens = t.tokens - 1
}

// addNewTokens is a private api(thus needs no locking). It should only ever be called by `tb.limit`
func (t *tb) addNewTokens() {
	now := time.Now().UTC()
	timeSinceUpdate := now.Sub(time.Unix(t.updatedAt, 0).UTC())
	newTokens := timeSinceUpdate.Seconds() * t.sendRate

	if newTokens > 1 {
		t.tokens = math.Min((t.tokens + newTokens), t.maxTokens)
		t.updatedAt = now.Unix()
		t.messagesDelivered = 0
	}
}
