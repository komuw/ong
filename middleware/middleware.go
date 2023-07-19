// Package middleware provides helpful functions that implement some common functionalities in http servers.
// A middleware is a function that takes in a [http.Handler] as one of its arguments and returns a [http.Handler]
//
// The middlewares [All], [Get], [Post], [Head], [Put] & [Delete] wrap other internal middleware.
// The effect of this is that the aforementioned middleware, in addition to their specialised functionality, will:
//
//  1. Add logID for traceability.
//  2. Add the "real" client IP address to the request context.
//  3. Add client TLS fingerprint to the request context.
//  4. Recover from panics in the wrappedHandler.
//  5. Log http requests and responses.
//  6. Rate limit requests by IP address.
//  7. Shed load based on http response latencies.
//  8. Handle automatic procurement/renewal of ACME tls certificates.
//  9. Redirect http requests to https.
//  10. Add some important HTTP security headers and assign them sensible default values.
//  11. Implement Cross-Origin Resource Sharing support(CORS).
//  12. Provide protection against Cross Site Request Forgeries(CSRF).
//  13. Attempt to provide protection against form re-submission when a user reloads an already submitted web form.
//  14. Implement http sessions.
package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"unicode"

	"github.com/komuw/ong/internal/acme"
	"golang.org/x/exp/slog"
)

const (
	// ongMiddlewareErrorHeader is a http header that is set by Ong
	// whenever any of it's middlewares return an error.
	// The logger & recoverer middleware will log the value of this header if it is set.
	//
	// An example, is when the Get middleware fails because it has been called with the wrong http method.
	// Or when the csrf middleware fails because a csrf token was not found for POST/DELETE/etc requests.
	ongMiddlewareErrorHeader = "Ong-Middleware-Error"
)

// Opts are the various parameters(optionals) that can be used to configure middlewares.
//
// Use either [New] or [WithOpts] to get a valid Opts.
type Opts struct {
	domain         string
	httpsPort      uint16
	allowedOrigins []string
	allowedMethods []string
	allowedHeaders []string
	secretKey      string
	strategy       ClientIPstrategy
	l              *slog.Logger
}

// New returns a new Opts.
// It panics on error.
//
// domain is the domain name of your website. It can be an exact domain, subdomain or wildcard.
//
// httpsPort is the tls port where http requests will be redirected to.
//
// allowedOrigins, allowedMethods, & allowedHeaders are used by the CORS middleware.
// If allowedOrigins is nil, all origins are allowed. You can also use * to allow all.
// If allowedMethods is nil, "GET", "POST", "HEAD" are allowed. Use * to allow all.
// If allowedHeaders is nil, "Origin", "Accept", "Content-Type", "X-Requested-With" are allowed. Use * to allow all.
//
// secretKey is used for securing signed data.
// It should be unique & kept secret.
// If it becomes compromised, generate a new one and restart your application using the new one.
//
// strategy is the algorithm to use when fetching the client's IP address; see [ClientIPstrategy].
// It is important to choose your strategy carefully, see the warning in [ClientIP].
//
// [ACME]: https://en.wikipedia.org/wiki/Automatic_Certificate_Management_Environment
// [letsencrypt]: https://letsencrypt.org/
func New(
	domain string,
	httpsPort uint16,
	allowedOrigins []string,
	allowedMethods []string,
	allowedHeaders []string,
	secretKey string,
	strategy ClientIPstrategy,
	l *slog.Logger,
) Opts {
	if err := acme.Validate(domain); err != nil {
		panic(err)
	}

	if strings.Contains(domain, "*") {
		// remove the `*` and `.`
		domain = domain[2:]
	}

	if err := checkSecretKey(secretKey); err != nil {
		panic(err)
	}

	return Opts{
		domain:         domain,
		httpsPort:      httpsPort,
		allowedOrigins: allowedOrigins,
		allowedMethods: allowedMethods,
		allowedHeaders: allowedHeaders,
		secretKey:      secretKey,
		strategy:       strategy,
		l:              l,
	}
}

// WithOpts returns a new Opts that has sensible defaults.
// See [New] for extra documentation.
func WithOpts(
	domain string,
	httpsPort uint16,
	secretKey string,
	strategy ClientIPstrategy,
	l *slog.Logger,
) Opts {
	return New(domain, httpsPort, nil, nil, nil, secretKey, strategy, l)
}

// allDefaultMiddlewares is a middleware that bundles all the default/core middlewares into one.
//
// example usage:
//
//	allDefaultMiddlewares(wh, WithOpts("example.com", 443, "secretKey", RightIpStrategy, log.New(os.Stdout, 10)))
func allDefaultMiddlewares(
	wrappedHandler http.Handler,
	o Opts,
) http.HandlerFunc {
	domain := o.domain
	httpsPort := o.httpsPort
	allowedOrigins := o.allowedOrigins
	allowedMethods := o.allowedOrigins
	allowedHeaders := o.allowedHeaders
	secretKey := o.secretKey
	strategy := o.strategy
	l := o.l

	// The way the middlewares are layered is:
	// 1.  trace on outer most since we need to add logID's earliest for use by inner middlewares.
	// 2.  clientIP on outer since client IP is needed by a couple of inner middlewares.
	// 3.  fingerprint because it is needed by recoverer & logger.
	// 4.  recoverer on the outer since we want it to watch all other middlewares.
	// 5.  logger since we would like to get logs as early in the lifecycle as possible.
	// 6.  rateLimiter since we want bad traffic to be filtered early.
	// 7.  loadShedder for the same reason.
	// 8.  acme needs to come before httpsRedirector because ACME challenge requests need to be handled under http(not https).
	// 9.  httpsRedirector since it can be cpu intensive, thus should be behind the ratelimiter & loadshedder.
	// 10. securityHeaders since we want some minimum level of security.
	// 11. cors since we might get pre-flight requests and we don't want those to go through all the middlewares for performance reasons.
	// 12. csrf since this one is a bit more involved perf-wise.
	// 13. Gzip since it is very involved perf-wise.
	// 14. reloadProtector, ideally I feel like it should come earlier but I'm yet to figure out where.
	// 15. session since we want sessions to saved as soon as possible.
	//
	// user ->
	//  trace ->
	//   clientIP ->
	//    fingerprint ->
	//     recoverer ->
	//      logger ->
	//       rateLimiter ->
	//        loadShedder ->
	//         acme ->
	//          httpsRedirector ->
	//           securityHeaders ->
	//            cors ->
	//             csrf ->
	//              Gzip ->
	//               reloadProtector ->
	//                session ->
	//                 actual-handler

	// We have disabled Gzip for now, since it is about 2.5times slower than no-gzip for a 50MB sample response.
	// see: https://github.com/komuw/ong/issues/85

	// acme(wrappedHandler http.Handler, domain, acmeEmail, acmeDirectoryUrl string)
	// acmeEmail , acmeDirectoryUrl

	return trace(
		clientIP(
			fingerprint(
				recoverer(
					logger(
						rateLimiter(
							loadShedder(
								acme.Handler(
									httpsRedirector(
										securityHeaders(
											cors(
												csrf(
													reloadProtector(
														session(
															wrappedHandler,
															secretKey,
															domain,
														),
														domain,
													),
													secretKey,
													domain,
												),
												allowedOrigins,
												allowedMethods,
												allowedHeaders,
											),
											domain,
										),
										httpsPort,
										domain,
									),
								),
							),
						),
						l,
					),
					l,
				),
			),
			strategy,
		),
		domain,
	)
}

// All is a middleware that allows all http methods.
//
// See the package documentation for the additional functionality provided by this middleware.
func All(wrappedHandler http.Handler, o Opts) http.HandlerFunc {
	return allDefaultMiddlewares(
		all(wrappedHandler),
		o,
	)
}

func all(wrappedHandler http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		wrappedHandler.ServeHTTP(w, r)
	}
}

// Get is a middleware that only allows http GET requests and http OPTIONS requests.
//
// See the package documentation for the additional functionality provided by this middleware.
func Get(wrappedHandler http.Handler, o Opts) http.HandlerFunc {
	return allDefaultMiddlewares(
		get(wrappedHandler),
		o,
	)
}

func get(wrappedHandler http.Handler) http.HandlerFunc {
	msg := "http method: %s not allowed. only allows http GET"
	return func(w http.ResponseWriter, r *http.Request) {
		// We do not need to allow `http.MethodOptions` here.
		// This is coz, the cors middleware has already handled that for us and it comes before the Get middleware.
		if r.Method != http.MethodGet {
			errMsg := fmt.Sprintf(msg, r.Method)
			w.Header().Set(ongMiddlewareErrorHeader, errMsg)
			http.Error(
				w,
				errMsg,
				http.StatusMethodNotAllowed,
			)
			return
		}

		wrappedHandler.ServeHTTP(w, r)
	}
}

// Post is a middleware that only allows http POST requests and http OPTIONS requests.
//
// See the package documentation for the additional functionality provided by this middleware.
func Post(wrappedHandler http.Handler, o Opts) http.HandlerFunc {
	return allDefaultMiddlewares(
		post(wrappedHandler),
		o,
	)
}

func post(wrappedHandler http.Handler) http.HandlerFunc {
	msg := "http method: %s not allowed. only allows http POST"
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			errMsg := fmt.Sprintf(msg, r.Method)
			w.Header().Set(ongMiddlewareErrorHeader, errMsg)
			http.Error(
				w,
				errMsg,
				http.StatusMethodNotAllowed,
			)
			return
		}

		wrappedHandler.ServeHTTP(w, r)
	}
}

// Head is a middleware that only allows http HEAD requests and http OPTIONS requests.
//
// See the package documentation for the additional functionality provided by this middleware.
func Head(wrappedHandler http.Handler, o Opts) http.HandlerFunc {
	return allDefaultMiddlewares(
		head(wrappedHandler),
		o,
	)
}

func head(wrappedHandler http.Handler) http.HandlerFunc {
	msg := "http method: %s not allowed. only allows http HEAD"
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			errMsg := fmt.Sprintf(msg, r.Method)
			w.Header().Set(ongMiddlewareErrorHeader, errMsg)
			http.Error(
				w,
				errMsg,
				http.StatusMethodNotAllowed,
			)
			return
		}

		wrappedHandler.ServeHTTP(w, r)
	}
}

// Put is a middleware that only allows http PUT requests and http OPTIONS requests.
//
// See the package documentation for the additional functionality provided by this middleware.
func Put(wrappedHandler http.Handler, o Opts) http.HandlerFunc {
	return allDefaultMiddlewares(
		put(wrappedHandler),
		o,
	)
}

func put(wrappedHandler http.Handler) http.HandlerFunc {
	msg := "http method: %s not allowed. only allows http PUT"
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			errMsg := fmt.Sprintf(msg, r.Method)
			w.Header().Set(ongMiddlewareErrorHeader, errMsg)
			http.Error(
				w,
				errMsg,
				http.StatusMethodNotAllowed,
			)
			return
		}

		wrappedHandler.ServeHTTP(w, r)
	}
}

// Delete is a middleware that only allows http DELETE requests and http OPTIONS requests.
//
// See the package documentation for the additional functionality provided by this middleware.
func Delete(wrappedHandler http.Handler, o Opts) http.HandlerFunc {
	return allDefaultMiddlewares(
		deleteH(wrappedHandler),
		o,
	)
}

// this is not called `delete` since that is a Go builtin func for deleting from maps.
func deleteH(wrappedHandler http.Handler) http.HandlerFunc {
	msg := "http method: %s not allowed. only allows http DELETE"
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			errMsg := fmt.Sprintf(msg, r.Method)
			w.Header().Set(ongMiddlewareErrorHeader, errMsg)
			http.Error(
				w,
				errMsg,
				http.StatusMethodNotAllowed,
			)
			return
		}

		wrappedHandler.ServeHTTP(w, r)
	}
}

// checkSecretKey assures that the secretKey has a minimum of desirable security properties.
func checkSecretKey(secretKey string) error {
	minLen := 6
	maxLen := 256
	if len(secretKey) < minLen {
		return fmt.Errorf("ong/middleware: secretKey size is less than minimum required of %d", minLen)
	}
	if len(secretKey) > maxLen {
		return fmt.Errorf("ong/middleware: secretKey size is more than maximum required of %d", maxLen)
	}

	hasDigit := 0
	hasSymbol := 0
	hasLowerCase := 0
	hasUpperCase := 0
	allZeros := true

	for _, r := range secretKey {
		if unicode.IsDigit(rune(r)) {
			hasDigit = hasDigit + 1
		}
		if unicode.IsPunct(rune(r)) {
			hasSymbol = hasSymbol + 1
		}
		if unicode.IsLower(rune(r)) {
			hasLowerCase = hasLowerCase + 1
		}
		if unicode.IsUpper(rune(r)) {
			hasUpperCase = hasUpperCase + 1
		}
		if r != rune(0) {
			allZeros = false
		}
	}

	expected := 1
	if hasDigit < expected {
		return fmt.Errorf("ong/middleware: secretKey should have at least %d digits", expected)
	}
	if hasSymbol < expected {
		return fmt.Errorf("ong/middleware: secretKey should have at least %d symbols", expected)
	}
	if hasLowerCase < expected {
		return fmt.Errorf("ong/middleware: secretKey should have at least %d lowercase characters", expected)
	}
	if hasUpperCase < expected {
		return fmt.Errorf("ong/middleware: secretKey should have at least %d uppercase characters", expected)
	}
	if allZeros {
		return fmt.Errorf("ong/middleware: secretKey cannot be all zeros")
	}

	return nil
}
