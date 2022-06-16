package middleware

import (
	"net/http"
	"strings"

	"golang.org/x/exp/slices"
)

const (
	// header is used by browsers when issuing a preflight request.
	acrmHeader = "Access-Control-Request-Method"
	// used by browsers when issuing a preflight request to let the server know which HTTP headers the client might send when the actual request is made
	acrhHeader   = "Access-Control-Request-Headers"
	originHeader = "Origin"
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
	// use `*` to allow all.
	allowedOrigins := []string{} // TODO: offer ability for library user to provide these.
	allowedWildcardOrigins := createWildcards(allowedOrigins)
	_allowedMethods := []string{
		// the spec by default allows this simple methods: GET, POST, HEAD.
		http.MethodGet,
		http.MethodPost,
		http.MethodHead,
	}
	allowedMethods := []string{} // TODO: offer ability for library users to augument these.
	for _, v := range _allowedMethods {
		allowedMethods = append(allowedMethods, strings.ToUpper(v))
	}

	// use `*` to allow all.
	_allowedHeaders := []string{}
	allowedHeaders := []string{} // TODO: offer ability for library user to provide these.
	for _, v := range _allowedHeaders {
		allowedHeaders = append(allowedHeaders, http.CanonicalHeaderKey(v))
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions && r.Header.Get(acrmHeader) != "" {
			// handle preflight request
			handlePreflight(w, r, allowedOrigins, allowedWildcardOrigins, allowedMethods, allowedHeaders)
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

func handlePreflight(
	w http.ResponseWriter,
	r *http.Request,
	allowedOrigins []string,
	allowedWildcardOrigins []wildcard,
	allowedMethods []string,
	allowedHeaders []string,
) {
	headers := w.Header()
	origin := r.Header.Get(originHeader)
	reqMethod := r.Header.Get(acrmHeader)
	reqHeader := r.Header.Get(acrhHeader)

	if r.Method != http.MethodOptions {
		// this is not a pre-flight request.
		return
	}

	// Always set Vary headers
	// see https://github.com/rs/cors/issues/10,
	//     https://github.com/rs/cors/commit/dbdca4d95feaa7511a46e6f1efb3b3aa505bc43f#commitcomment-12352001
	headers.Add(varyHeader, originHeader)
	headers.Add(varyHeader, acrmHeader)
	headers.Add(varyHeader, acrhHeader)

	if origin == "" {
		return
	}

	if !isOriginAllowed(r, origin, allowedOrigins, allowedWildcardOrigins) {
		return
	}

	if !isMethodAllowed(reqMethod, allowedMethods) {
		return
	}

	if !areHeadersAllowed(reqHeader, allowedHeaders) {
		return
	}
}

type wildcard struct {
	prefix string
	suffix string
	len    int
}

func (w wildcard) match(s string) bool {
	return len(s) >= w.len &&
		strings.HasPrefix(s, w.prefix) &&
		strings.HasSuffix(s, w.suffix)
}

func createWildcards(allowedOrigins []string) []wildcard {
	allowedWildcardOrigins := []wildcard{}
	for _, origin := range allowedOrigins {
		if i := strings.IndexByte(origin, '*'); i >= 0 {
			// Split the origin in two: start and end string without the *
			prefix := origin[0:i]
			suffix := origin[i+1:]
			w := wildcard{
				prefix: prefix,
				suffix: suffix,
				len:    len(prefix) + len(suffix),
			}
			allowedWildcardOrigins = append(allowedWildcardOrigins, w)
		}
	}

	return allowedWildcardOrigins
}

func isOriginAllowed(
	r *http.Request,
	origin string,
	allowedOrigins []string,
	allowedWildcardOrigins []wildcard,
) bool {
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

func isMethodAllowed(method string, allowedMethods []string) bool {
	if len(allowedMethods) == 0 {
		// If no method allowed, always return false, even for preflight request
		return false
	}

	method = strings.ToUpper(method)
	if method == http.MethodOptions {
		// Always allow preflight requests
		return true
	}

	for _, m := range allowedMethods {
		if m == method {
			return true
		}
	}
	return false
}

func areHeadersAllowed(reqHeader string, allowedHeaders []string) bool {
	// Access-Control-Request-Headers: X-PINGOTHER, Content-Type
	requestedHeaders := strings.Split(reqHeader, ",")

	if len(requestedHeaders) == 0 {
		return true
	}

	if slices.Contains(allowedHeaders, "*") {
		return true
	}

	for _, header := range requestedHeaders {
		header = http.CanonicalHeaderKey(header)
		found := false
		for _, h := range allowedHeaders {
			if h == header {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func handleActualRequest(w http.ResponseWriter, r *http.Request) {
}
