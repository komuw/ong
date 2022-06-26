package middleware

import (
	"fmt"
	"net/http"
	"time"
)

type latency struct {
	duration time.Duration
	at       time.Time
}

func (l latency) String() string {
	return fmt.Sprintf("{dur: %s, at: %s}", l.duration, l.at)
}

type latencyQueue []latency

// TODO: with the algorithm we are gong with; this looks like a loadShedder rather than a rateLimiter.

// RateLimiter is a middleware that rate limits requests based on the IP address.
func RateLimiter(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	lq := latencyQueue{} // TODO, we need to purge this queue regurlary

	// The minimum number of past requests that have to be available, in the last `samplingPeriod` seconds for us to make a decision.
	// If there were fewer requests in the `samplingPeriod`, then we do decide to let things continue without ratelimiting.
	minSampleSize := 10
	samplingPeriod := 10 * time.Second
	// The p99 latency(in milliSeconds) at which point we start dropping requests.
	breachLatency := 3 * time.Second

	_ = minSampleSize
	_ = samplingPeriod
	_ = breachLatency // TODO: remove this.

	return func(w http.ResponseWriter, r *http.Request) {
		startReq := time.Now().UTC()
		defer func() {
			endReq := time.Now().UTC()
			durReq := endReq.Sub(startReq)
			lq = append(lq, latency{duration: durReq, at: endReq})

			fmt.Println("\n\t lq: ", lq)
		}()

		wrappedHandler(w, r)
	}
}

// func fetchIP(remoteAddr string) string {
// 	// the documentation of `http.Request.RemoteAddr` says:
// 	// RemoteAddr is not filled in by ReadRequest and has no defined format.
// 	// So we cant rely on it been present, or having a given format.
// 	// Although, net/http makes a good effort of availing it & in a standard format.
// 	//
// 	ipAddr := strings.Split(remoteAddr, ":")
// 	return ipAddr[0]
// }

// TODO:
//
// func LoadShed(wrappedHandler http.HandlerFunc) http.HandlerFunc {
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		start := time.Now()

// 		// check latency from store over the past X minutes.
// 		// if 99th percentile is greater than configured value,
// 		// drop the request and set a `Retry-After` http header.
// 		// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Retry-After
//
// 		defer func() {
//          Do not record latency into the store if this response is not coming from the actual target handler.
//
// 			latency := time.Since(start).Milliseconds()
// 			// store latency in store.
// 		}()

// 		wrappedHandler(w, r)
// 	}
// }
