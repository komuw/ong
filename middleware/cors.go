package middleware

import (
	"net/http"
	"strings"

	"golang.org/x/exp/slices"
)

// Most of the code here is insipired(or taken from) by:
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
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			// handle preflight request
			handlePreflight(w, r)
			// Preflight requests are standalone and should stop the chain as some other
			// middleware may not handle OPTIONS requests correctly. One typical example
			// is authentication middleware ; OPTIONS requests won't carry authentication headers.
			w.WriteHeader(http.StatusNoContent)
		} else {
			// handle actual request
			handleActualRequest(w, r)
			wrappedHandler(w, r)
		}
	}
}

func handlePreflight(w http.ResponseWriter, r *http.Request) {
	headers := w.Header()
	origin := r.Header.Get("Origin")

	if r.Method != http.MethodOptions {
		// this is not a pre-flight request.
		return
	}

	// Always set Vary headers
	// see https://github.com/rs/cors/issues/10,
	//     https://github.com/rs/cors/commit/dbdca4d95feaa7511a46e6f1efb3b3aa505bc43f#commitcomment-12352001
	headers.Add("Vary", "Origin")
	headers.Add("Vary", "Access-Control-Request-Method")
	headers.Add("Vary", "Access-Control-Request-Headers")

	if origin == "" {
		return
	}

	if !isOriginAllowed(r, origin) {
		return
	}
}

type wildcard struct {
	prefix string
	suffix string
}

func (w wildcard) match(s string) bool {
	return len(s) >= len(w.prefix)+len(w.suffix) && strings.HasPrefix(s, w.prefix) && strings.HasSuffix(s, w.suffix)
}

var (
	allowedOrigins         = []string{}
	allowedWildcardOrigins = []wildcard{}
)

func createWildcards() {
	for _, origin := range allowedOrigins {
		if i := strings.IndexByte(origin, '*'); i >= 0 {
			// Split the origin in two: start and end string without the *
			w := wildcard{origin[0:i], origin[i+1:]}
			allowedWildcardOrigins = append(allowedWildcardOrigins, w)
		}
	}
}

func isOriginAllowed(r *http.Request, origin string) bool {
	if slices.Contains(allowedOrigins, "*") {
		return true
	}

	origin = strings.ToLower(origin)
	for _, o := range allowedOrigins {
		if o == origin {
			return true
		}
	}

	for _, w := range allowedWildcardOrigins {
		if w.match(origin) {
			return true
		}
	}

	return false
}

func handleActualRequest(w http.ResponseWriter, r *http.Request) {
}
