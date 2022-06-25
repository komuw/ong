package middleware

import (
	"fmt"
	"net/http"
	"strings"
)

// RateLimiter is a middleware that rate limits requests based on the IP address.
func RateLimiter(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("r.RemoteAddr: ", r.RemoteAddr)
		fmt.Println("r.headers: ", r.Header)
		fmt.Println("r.Method: ", r.Method)
		ip := fetchIP(r.RemoteAddr)
		fmt.Println("ip: ", ip)

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
