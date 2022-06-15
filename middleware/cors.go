package middleware

import "net/http"

// Most of the code here is insipired by:
//   (a) https://github.com/gofiber/fiber whose license(MIT) can be found here: https://github.com/rs/cors/blob/v1.8.2/LICENSE

// Cross-Origin Resource Sharing (CORS) is an HTTP-header based mechanism that allows a server to
// indicate any origins (domain/scheme/port) other than its own from which a browser should permit loading resources.
//
// CORS also relies on a mechanism by which browsers make a "preflight" request to the server hosting the cross-origin resource,
// in order to check that the server will permit the actual request.
// In that preflight, the browser sends headers that indicate the HTTP method and headers that will be used in the actual request.
//
// An origin is identified by a triple: scheme, fully qualified hostname and port.
// `http://example.com` and `https://example.com` are different origins(http vs https)
//
// - https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS

// Cors is a middleware to implement Cross-Origin Resource Sharing support.
func Cors(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		wrappedHandler(w, r)
	}
}
