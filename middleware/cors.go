package middleware

import "net/http"

// Most of the code here is insipired by:
//   (a) https://github.com/gofiber/fiber whose license(MIT) can be found here: https://github.com/rs/cors/blob/v1.8.2/LICENSE

// Cross-Origin Resource Sharing (CORS) is an HTTP-header based mechanism that allows a server to
// indicate any origins (domain/scheme/port) other than its own from which a browser should permit loading resources.

// Cors is a middleware to implement Cross-Origin Resource Sharing support.
func Cors(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		wrappedHandler(w, r)
	}
}
