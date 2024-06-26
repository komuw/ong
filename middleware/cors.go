package middleware

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"
)

// Some of the code here is inspired(or taken from) by:
//   (a) https://github.com/gofiber/fiber whose license(MIT) can be found here: https://github.com/rs/cors/blob/v1.8.2/LICENSE

// Cross-Origin Resource Sharing (CORS) is an HTTP-header based mechanism that allows a server to
// indicate any origins (scheme/domain/port) other than its own from which a browser should permit loading resources.
//
// CORS also relies on a mechanism by which browsers make a "preflight" request to the server hosting the cross-origin resource,
// in order to check that the server will permit the actual request.
// In that preflight, the browser sends headers that indicate the HTTP method and headers that will be used in the actual request.
//
// An origin is identified by a triple: scheme, fully qualified hostname and port.
// `http://example.com` and `https://example.com` are different origins(http vs https)
//
// - https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS
// - https://jub0bs.com/posts/2023-02-08-fearless-cors
// - https://fetch.spec.whatwg.org/#cors-preflight-request <- The fetch standard is also the canonical reference for CORS. YES.

const (
	// header is used by browsers when issuing a preflight request.
	acrmHeader = "Access-Control-Request-Method"
	// used by browsers when issuing a preflight request to let the server know which HTTP headers the client might send when the actual request is made
	acrhHeader   = "Access-Control-Request-Headers"
	originHeader = "Origin"
	acaoHeader   = "Access-Control-Allow-Origin"
	acamHeader   = "Access-Control-Allow-Methods"
	acahHeader   = "Access-Control-Allow-Headers"
	// header that if set to true, allows client & server to include credentials in cross-origin-requests.
	// credentials are cookies, authorization headers, or tls client certificates
	// The only valid value of this header is `true`(`false` is not valid, omit the header entirely instead.)
	acacHeader = "Access-Control-Allow-Credentials"
	// header to allow CORS to resources in a private network(eg behind a VPN)
	// you can set this header to `true` when you receive a preflight request if you want to allow access.
	// Otherwise omit it entirely(as we will in this library).
	// If we ever implement this, read; https://jub0bs.com/posts/2023-02-08-fearless-cors/#3-provide-support-for-private-network-access
	acrpnHeader = "Access-Control-Request-Private-Network"
	_           = acrpnHeader
	// acmaHeader stores how long(in seconds) the results of a preflight request can be cached.
	acmaHeader = "Access-Control-Max-Age"
	// allows a server to indicate which response headers should be made available to scripts running in the browser for cross-origin-requests.
	// by default only the cors-safelisted response headers(https://developer.mozilla.org/en-US/docs/Glossary/CORS-safelisted_response_header) are allowed.
	// For this library, we won't allow any other headers to be exposed; which means we will omit setting this header entirely.
	acehHeader = "Access-Control-Expose-Headers"
	_          = acehHeader
)

// cors is a middleware to implement Cross-Origin Resource Sharing support.
//
// If allowedOrigins is nil, domain and its www variant are used. You can also use * to allow all.
// If allowedMethods is nil, "GET", "POST", "HEAD" are allowed. Use * to allow all.
// If allowedHeaders is nil, "Origin", "Accept", "Content-Type", "X-Requested-With" are allowed. Use * to allow all.
//
// This func assumes that the arguments to it are valid. Use [github.com/komuw/ong/config.New] to ensure that the arguments are valid.
func cors(
	wrappedHandler http.Handler,
	allowedOrigins []string,
	allowedMethods []string,
	allowedHeaders []string,
	allowCredentials bool,
	corsCacheDuration time.Duration,
	domain string,
) http.HandlerFunc {
	// Validation of arguments should already have happened in `github.com/komuw/ong/config`
	allowedOrigins, allowedWildcardOrigins := getOrigins(allowedOrigins, domain)
	allowedMethods = getMethods(allowedMethods)
	allowedHeaders = getHeaders(allowedHeaders)

	return func(w http.ResponseWriter, r *http.Request) {
		if (r.Method == http.MethodOptions) && (r.Header.Get(acrmHeader) != "") && (r.Header.Get(originHeader) != "") {
			/*
				The Fetch standard says that a preflight request is one that:
				(a) uses the OPTIONS method
				(b) includes an Origin header,
				(c) includes an Access-Control-Request-Method header.
				- https://fetch.spec.whatwg.org/#cors-preflight-request
				- https://jub0bs.com/posts/2023-02-08-fearless-cors/#4-categorise-requests-correctly
			*/
			// handle preflight request
			handlePreflight(w, r, allowedOrigins, allowedWildcardOrigins, allowedMethods, allowedHeaders, allowCredentials, int(corsCacheDuration.Seconds()))
			// Preflight requests are standalone and should stop the chain as some other
			// middleware may not handle OPTIONS requests correctly. One typical example
			// is authentication middleware ; OPTIONS requests won't carry authentication headers.
			w.WriteHeader(http.StatusNoContent)
		} else {
			// handle actual request
			handleActualRequest(w, r, allowedOrigins, allowedWildcardOrigins, allowedMethods, allowCredentials)
			wrappedHandler.ServeHTTP(w, r)
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
	allowCredentials bool,
	corsCacheDuration int,
) {
	headers := w.Header()
	origin := r.Header.Get(originHeader)
	// According to the CORS/Fetch standard, method names are case-sensitive. So do not uppercase/lowercase reqMethod.
	reqMethod := r.Header.Get(acrmHeader) // note this is different from the one in `handleActualRequest`
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

	allow, allowAll := isOriginAllowed(origin, allowedOrigins, allowedWildcardOrigins)
	if !allow {
		return
	}

	if !isMethodAllowed(reqMethod, allowedMethods) {
		return
	}

	if !areHeadersAllowed(reqHeader, allowedHeaders) {
		return
	}

	// upto this point, it means we are going to allow the preflight.
	// we need to set appropriate headers.
	// (a) allowed origin.
	// (b) allowed methods.
	// (c) allowed headers.
	// (d) cache.

	// (a)
	if allowAll {
		headers.Set(acaoHeader, "*")
	} else {
		headers.Set(acaoHeader, origin)
	}

	// (b)
	// spec says we can return just one method instead of all the supported ones.
	// that one method has to be the one that came in via the `acrmHeader`
	headers.Set(
		acamHeader,
		// According to the CORS/Fetch standard, method names are case-sensitive.
		// Thus if preflight has `put`, the header should have `put`(not `PUT`) else browser will show error.
		//
		// - https://fetch.spec.whatwg.org/#methods
		// - https://jub0bs.com/posts/2023-02-08-fearless-cors/#violation-of-the-case-sensitivity-of-method-names
		reqMethod,
	)

	// (c)
	if len(reqMethod) > 0 {
		// spec says we can return the headers that came in via the `acrhHeader`
		headers.Set(acahHeader, reqHeader)
		// we do not set the `acacHeader`
	}

	// (d)
	headers.Set(acmaHeader, fmt.Sprintf("%d", corsCacheDuration))

	// (e)
	if allowCredentials {
		headers.Set(acacHeader, "true")
	}
}

func handleActualRequest(
	w http.ResponseWriter,
	r *http.Request,
	allowedOrigins []string,
	allowedWildcardOrigins []wildcard,
	allowedMethods []string,
	allowCredentials bool,
) {
	headers := w.Header()
	origin := r.Header.Get(originHeader)
	reqMethod := r.Method // note this is different from the one in `handlePreflight`

	// Always set Vary, see https://github.com/rs/cors/issues/10
	headers.Add(varyHeader, originHeader)

	if origin == "" {
		return
	}

	allow, allowAll := isOriginAllowed(origin, allowedOrigins, allowedWildcardOrigins)
	if !allow {
		return
	}

	if !isMethodAllowed(reqMethod, allowedMethods) {
		return
	}

	// we need to set appropriate headers.
	// (a) allowed origin.
	if allowAll {
		headers.Set(acaoHeader, "*")
	} else {
		headers.Set(acaoHeader, origin)
	}

	// (b)
	if allowCredentials {
		headers.Set(acacHeader, "true")
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

func isOriginAllowed(
	origin string,
	allowedOrigins []string,
	allowedWildcardOrigins []wildcard,
) (allow, allowAll bool) {
	if slices.Contains(allowedOrigins, "*") {
		return true, true
	}

	origin = strings.ToLower(origin)
	for _, o := range allowedOrigins {
		if o == origin {
			return true, false
		}
	}

	for _, w := range allowedWildcardOrigins {
		if w.match(origin) {
			return true, false
		}
	}

	return false, false
}

func isMethodAllowed(method string, allowedMethods []string) bool {
	// todo: allow ability for users of library to set empty allowedMethods
	//       ie, `len(allowedMethods) == 0` which would disallow all methods.

	if slices.Contains(allowedMethods, "*") {
		return true
	}

	// The spec talks of normalizing some methods(`DELETE`, `GET`, `HEAD`, `OPTIONS`, `POST`, `PUT`) to uppercase.
	// This library normalizes all methods not just the ones mentioned in the spec.
	// https://fetch.spec.whatwg.org/#concept-method-normalize
	method = strings.ToUpper(method)
	if method == http.MethodOptions {
		// Always allow preflight requests
		return true
	}

	return slices.Contains(allowedMethods, method)
}

func areHeadersAllowed(reqHeader string, allowedHeaders []string) bool {
	// Access-Control-Request-Headers: X-PINGOTHER, Content-Type
	requestedHeaders := strings.FieldsFunc(
		reqHeader,
		func(c rune) bool {
			// reqHeader could be either of:
			//   - `"X-PINGOTHER,Content-Type"`
			//   - `"X-PINGOTHER, Content-Type"`
			return c == ',' || c == ' '
		},
	)

	if len(requestedHeaders) == 0 || len(reqHeader) == 0 {
		return true
	}

	if slices.Contains(allowedHeaders, "*") {
		return true
	}

	// requestedHeaders should be a subset of allowedHeaders for us to return true.
	// ie, allowedHeaders should be a superset of requestedHeaders.
	for _, header := range requestedHeaders {
		header = http.CanonicalHeaderKey(header)
		found := slices.Contains(allowedHeaders, header)
		if !found {
			return false
		}
	}

	return true
}

func getOrigins(ao []string, domain string) (allowedOrigins []string, allowedWildcardOrigins []wildcard) {
	if len(ao) == 0 {
		if strings.Contains(domain, "*") { // `acme.Validate(domain)` should have been called prior.
			domain = domain[2:] // remove the `*` and `.`
		}
		domains := []string{"https://" + domain}
		if !strings.HasPrefix(domain, "www") && strings.Count(domain, ".") == 1 {
			domains = append(domains, "https://www."+domain) // as a special case, add `www`
		}
		return domains, []wildcard{}
	}

	canon := []string{}
	for _, v := range ao {
		canon = append(canon, strings.ToLower(v))
	}
	allowedOrigins = canon

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
	return allowedOrigins, allowedWildcardOrigins
}

func getMethods(am []string) []string {
	if len(am) == 0 {
		return []string{
			// the spec by default allows this simple methods for cross-origin-requests: GET, POST, HEAD.
			// https://fetch.spec.whatwg.org/#methods
			strings.ToUpper(http.MethodGet),
			strings.ToUpper(http.MethodPost),
			strings.ToUpper(http.MethodHead),
		}
	}

	if slices.Contains(am, "*") {
		return []string{"*"}
	}

	allowedMethods := []string{}
	for _, v := range am {
		allowedMethods = append(allowedMethods, strings.ToUpper(v))
	}

	return allowedMethods
}

func getHeaders(ah []string) []string {
	if len(ah) == 0 {
		// Use the list of CORS safelisted request headers.
		// - https://developer.mozilla.org/en-US/docs/Glossary/CORS-safelisted_request_header
		// - https://fetch.spec.whatwg.org/#no-cors-safelisted-request-header-name
		// - https://fetch.spec.whatwg.org/#privileged-no-cors-request-header-name
		return []string{
			http.CanonicalHeaderKey("Accept"),
			http.CanonicalHeaderKey("Accept-Language"),
			http.CanonicalHeaderKey("Content-Language"),
			http.CanonicalHeaderKey("Content-Type"),
			http.CanonicalHeaderKey("Range"),
		}
	}

	if slices.Contains(ah, "*") {
		return []string{"*"}
	}

	allowedHeaders := []string{}
	for _, v := range ah {
		allowedHeaders = append(allowedHeaders, http.CanonicalHeaderKey(v))
	}

	return allowedHeaders
}
