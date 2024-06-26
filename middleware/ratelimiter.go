package middleware

import (
	"fmt"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/komuw/ong/config"
)

// Some of the code here is inspired by(or taken from):
//   (a) https://github.com/komuw/naz/blob/v0.8.1/naz/ratelimiter.py whose license(MIT) can be found here: https://github.com/komuw/naz/blob/v0.8.1/LICENSE.txt

// rateLimiter is a middleware that limits requests by IP address.
func rateLimiter(
	wrappedHandler http.Handler,
	rateLimit float64,
) http.HandlerFunc {
	rl := newRl()
	const retryAfter = 15 * time.Minute

	if rateLimit < 1.0 {
		rateLimit = config.DefaultRateLimit
	}

	return func(w http.ResponseWriter, r *http.Request) {
		rl.reSize()

		host := ClientIP(r)
		tb := rl.get(host, rateLimit)

		if !tb.allow() {
			err := fmt.Errorf("ong/middleware/ratelimiter: rate limited, retry after %s", retryAfter)
			w.Header().Set(ongMiddlewareErrorHeader, err.Error())
			w.Header().Set(retryAfterHeader, fmt.Sprintf("%d", int(retryAfter.Seconds()))) // header should be in seconds(decimal-integer).
			http.Error(
				w,
				err.Error(),
				http.StatusTooManyRequests,
			)
			return
		}

		// Note: We also limit max body size using `http.MaxBytesHandler`
		// See: ong/server

		// todo: also maybe add another limiter for IP subnet.
		//      see limitation: https://github.com/komuw/ong/issues/17#issuecomment-1114551281

		wrappedHandler.ServeHTTP(w, r)
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
}

func newTb(sendRate float64) *tb {
	return &tb{
		sendRate:  sendRate,
		maxTokens: sendRate,
		tokens:    sendRate,
		// if sendRate is in req/Sec, delayForTokens ought to be in seconds.
		// if sendRate is in req/ms, delayForTokens ought to be in ms.
		delayForTokens: 1 * time.Second,
		updatedAt:      time.Now().UTC().Unix(),
	}
}

func (t *tb) allow() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	alw := true
	if t.tokens < 1 {
		alw = false
	}

	if alw {
		t.tokens = t.tokens - 1
	} else {
		t.limit()
	}

	return alw
}

// limit is a private api(thus needs no locking). It should only ever be called by `tb.allow`
// +checklocksignore
func (t *tb) limit() {
	for t.tokens < 1 {
		t.addNewTokens()
		time.Sleep(t.delayForTokens)
	}
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
	}
}
