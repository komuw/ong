package middleware

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

func fetchIP(remoteAddr string) string {
	// the documentation of `http.Request.RemoteAddr` says:
	// RemoteAddr is not filled in by ReadRequest and has no defined format.
	// So we cant rely on it been present, or having a given format.
	// Although, net/http makes a good effort of availing it & in a standard format.
	//
	ipAddr := strings.Split(remoteAddr, ":")
	return ipAddr[0]
}

// Most of the code here is insipired by(or taken from):
//   (a) https://github.com/komuw/naz/blob/v0.8.1/naz/ratelimiter.py whose license(MIT) can be found here: https://github.com/komuw/naz/blob/v0.8.1/LICENSE.txt

// RateLimiter is a middleware that limits requests by IP address.
func RateLimiter(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	r := rate.Limit(10) // 10req/sec
	burst := 2          // permit burst of 2 reqs
	l := rate.NewLimiter(r, burst)
	_ = l

	return func(w http.ResponseWriter, r *http.Request) {
		ip := fetchIP(r.RemoteAddr)
		fmt.Println("ip: ", ip)

		l.Wait(context.Background())

		wrappedHandler(w, r)
	}
}

// tb is a simple implementation of the token bucket rate limiting algorithm
// https://en.wikipedia.org/wiki/Token_bucket
type tb struct {
	sendRate       float64 // In req/seconds
	maxTokens      float64
	tokens         float64
	delayForTokens time.Duration
	// updatedAt is the time at which this operation took place.
	// We could have ideally used a `time.Time` as its type; but we wanted the latency struct to be minimal in size.
	updatedAt         int64
	messagesDelivered float64
	effectiveSendRate float64
}

func newRl(sendRate float64) *tb {
	return &tb{
		sendRate:          sendRate,
		maxTokens:         sendRate,
		tokens:            sendRate,
		delayForTokens:    1 * time.Second,
		updatedAt:         time.Now().UTC().Unix(),
		messagesDelivered: 0,
		effectiveSendRate: 0.00,
	}
}

func (r *tb) add_new_tokens() {
	now := time.Now().UTC()
	timeSinceUpdate := now.Sub(time.Unix(r.updatedAt, 0).UTC())
	r.effectiveSendRate = r.messagesDelivered / timeSinceUpdate.Seconds()
	newTokens := timeSinceUpdate.Seconds() * r.sendRate

	if newTokens > 1 {
		r.tokens = math.Min((r.tokens + newTokens), r.maxTokens)
		r.updatedAt = now.Unix()
		r.messagesDelivered = 0
	}
}

func (r *tb) limit() {
	for r.tokens < 1 {
		r.add_new_tokens()
		time.Sleep(r.delayForTokens)
	}
	r.messagesDelivered = r.messagesDelivered + 1
	r.tokens = r.tokens - 1
}
