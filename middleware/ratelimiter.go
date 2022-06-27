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

// Most of the code here is insipired by(or taken from):
//   (a) https://github.com/komuw/naz/blob/v0.8.1/naz/ratelimiter.py whose license(MIT) can be found here: https://github.com/komuw/naz/blob/v0.8.1/LICENSE.txt

type rl struct {
	sendRate          float64
	maxTokens         float64
	tokens            float64
	delayForTokens    time.Duration
	updatedAt         time.Time // change to int64 like in `latency.at`
	messagesDelivered float64
	effectiveSendRate float64
}

func newRl(sendRate float64) *rl {
	return &rl{
		sendRate:          sendRate,
		maxTokens:         sendRate,
		tokens:            sendRate,
		delayForTokens:    1 * time.Second,
		updatedAt:         time.Now().UTC(),
		messagesDelivered: 0,
		effectiveSendRate: 0.00,
	}
}

func (r *rl) add_new_tokens() {
	now := time.Now().UTC()
	timeSinceUpdate := now.Sub(r.updatedAt)
	r.effectiveSendRate = r.messagesDelivered / timeSinceUpdate.Seconds()
	newTokens := timeSinceUpdate.Seconds() * r.sendRate

	if newTokens > 1 {
		r.tokens = math.Min((r.tokens + newTokens), r.maxTokens)
		r.updatedAt = now
		r.messagesDelivered = 0
	}
}

func (r *rl) limit() {
	for r.tokens < 1 {
		r.add_new_tokens()
		time.Sleep(r.delayForTokens)
	}
	r.messagesDelivered = r.messagesDelivered + 1
	r.tokens = r.tokens - 1
}

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

func fetchIP(remoteAddr string) string {
	// the documentation of `http.Request.RemoteAddr` says:
	// RemoteAddr is not filled in by ReadRequest and has no defined format.
	// So we cant rely on it been present, or having a given format.
	// Although, net/http makes a good effort of availing it & in a standard format.
	//
	ipAddr := strings.Split(remoteAddr, ":")
	return ipAddr[0]
}
