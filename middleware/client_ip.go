package middleware

import (
	"fmt"
	"net"
	"net/http"
)

func clientIP(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() { wrappedHandler(w, r) }()

		host, port, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			return
		}

		r.RemoteAddr = fmt.Sprintf("%s:%s", host, port)
	}
}
