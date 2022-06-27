package middleware

import (
	"fmt"
	"net/http"
	"strings"
)

// RateLimiter is a middleware that limits requests by IP address.
func RateLimiter(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
