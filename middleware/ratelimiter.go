package middleware

import (
	"fmt"
	"math"
	"net/http"
	"sync"
	"time"
)

// Most of the code here is insipired by(or taken from):
//   (a) https://github.com/komuw/naz/blob/v0.8.1/naz/ratelimiter.py whose license(MIT) can be found here: https://github.com/komuw/naz/blob/v0.8.1/LICENSE.txt

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
	mu sync.Mutex // protects all the other fields.

	// +checklocks:mu
	sendRate float64 // In req/seconds
	// +checklocks:mu
	maxTokens float64
	// +checklocks:mu
	tokens float64
	// +checklocks:mu
	delayForTokens time.Duration
	// updatedAt is the time at which this operation took place.
	// We could have ideally used a `time.Time` as its type; but we wanted the latency struct to be minimal in size.
	// +checklocks:mu
	updatedAt int64
	// +checklocks:mu
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
// +checklocksignore
func (t *tb) limit() {
	for t.tokens < 1 {
		t.addNewTokens()
		time.Sleep(t.delayForTokens)
	}
	t.messagesDelivered = t.messagesDelivered + 1
	t.tokens = t.tokens - 1
}

// addNewTokens is a private api(thus needs no locking). It should only ever be called by `tb.limit`
// +checklocksignore
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

// //////////////////////////////////////////////////////
type mutexLimiter struct {
	sync.Mutex
	last       time.Time
	sleepFor   time.Duration
	perRequest time.Duration
	maxSlack   time.Duration
}

// newMutexBased returns a new atomic based limiter.
func newMutexBased(rate int) *mutexLimiter {
	var (
		slack int           = 10
		per   time.Duration = 1 * time.Second
	)

	perRequest := per / time.Duration(rate)
	fmt.Println("\t perRequest: ", perRequest)

	l := &mutexLimiter{
		perRequest: perRequest,
		maxSlack:   -1 * time.Duration(slack) * perRequest,
	}
	return l
}

// Take blocks to ensure that the time spent between multiple
// Take calls is on average per/rate.
func (t *mutexLimiter) Take() time.Time {
	t.Lock()
	defer t.Unlock()

	now := time.Now()

	// If this is our first request, then we allow it.
	if t.last.IsZero() {
		t.last = now
		return t.last
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
	if t.sleepFor > 0 {
		time.Sleep(t.sleepFor)
		t.last = now.Add(t.sleepFor)
		t.sleepFor = 0
	} else {
		t.last = now
	}

	return t.last
}
