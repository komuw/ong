package middleware

import "net/http"

// Cross-Origin Resource Sharing (CORS) is an HTTP-header based mechanism that allows a server to
// indicate any origins (domain/scheme/port) other than its own from which a browser should permit loading resources.

// Cors is a middleware to implement Cross-Origin Resource Sharing support.
func Cors(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		wrappedHandler(w, r)
	}
}
