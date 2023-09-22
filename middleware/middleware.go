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
//  6. Try and prevent path traversal attack.
//  7. Rate limit requests by IP address.
//  8. Shed load based on http response latencies.
//  9. Handle automatic procurement/renewal of ACME tls certificates.
//  10. Redirect http requests to https.
//  11. Add some important HTTP security headers and assign them sensible default values.
//  12. Implement Cross-Origin Resource Sharing support(CORS).
//  13. Provide protection against Cross Site Request Forgeries(CSRF).
//  14. Attempt to provide protection against form re-submission when a user reloads an already submitted web form.
//  15. Implement http sessions.
package middleware

import (
	"fmt"
	"net/http"

	"github.com/komuw/ong/config"
	"github.com/komuw/ong/internal/acme"
)

const (
	// ongMiddlewareErrorHeader is a http header that is set by Ong
	// whenever any of it's middlewares return an error.
	// The logger & recoverer middleware will log the value of this header if it is set.
	//
	// An example, is when the Get middleware fails because it has been called with the wrong http method.
	// Or when the csrf middleware fails because a csrf token was not found for POST/DELETE/etc requests.
	ongMiddlewareErrorHeader = "Ong-Middleware-Error"

	allowHeader = "Allow"
)

// allDefaultMiddlewares is a middleware that bundles all the default/core middlewares into one.
//
// example usage:
//
//	allDefaultMiddlewares(wh, WithOpts("example.com", 443, "super-h@rd-Pas1word", RightIpStrategy, log.New(os.Stdout, 10)))
func allDefaultMiddlewares(
	wrappedHandler http.Handler,
	o config.Opts,
) http.HandlerFunc {
	domain := o.Domain
	httpsPort := o.HttpsPort
	secretKey := o.SecretKey
	strategy := o.Strategy
	l := o.Logger

	// logger
	rateShedSamplePercent := o.RateShedSamplePercent

	// ratelimit
	rateLimit := o.RateLimit

	// loadshed
	loadShedSamplingPeriod := o.LoadShedSamplingPeriod
	loadShedMinSampleSize := o.LoadShedMinSampleSize
	loadShedBreachLatency := o.LoadShedBreachLatency

	// cors
	allowedOrigins := o.AllowedOrigins
	allowedMethods := o.AllowedOrigins
	allowedHeaders := o.AllowedHeaders
	allowCredentials := o.AllowCredentials
	corsCacheDuration := o.CorsCacheDuration

	// csrf
	csrfTokenDuration := o.CsrfTokenDuration

	// session
	sessionCookieDuration := o.SessionCookieDuration

	// The way the middlewares are layered is:
	// 1.  trace on outer most since we need to add logID's earliest for use by inner middlewares.
	// 2.  clientIP on outer since client IP is needed by a couple of inner middlewares.
	// 3.  fingerprint because it is needed by recoverer & logger.
	// 4.  recoverer on the outer since we want it to watch all other middlewares.
	// 5.  logger since we would like to get logs as early in the lifecycle as possible.
	// 6.  traversal comes after logger since we would want to log the actual initial path requested.
	// 7.  rateLimiter since we want bad traffic to be filtered early.
	// 8.  loadShedder for the same reason.
	// 9.  acme needs to come before httpsRedirector because ACME challenge requests need to be handled under http(not https).
	// 10.  httpsRedirector since it can be cpu intensive, thus should be behind the ratelimiter & loadshedder.
	// 11. securityHeaders since we want some minimum level of security.
	// 12. cors since we might get pre-flight requests and we don't want those to go through all the middlewares for performance reasons.
	// 13. csrf since this one is a bit more involved perf-wise.
	// 14. Gzip since it is very involved perf-wise.
	// 15. reloadProtector, ideally I feel like it should come earlier but I'm yet to figure out where.
	// 16. session since we want sessions to saved as soon as possible.
	//
	// user ->
	//  trace ->
	//   clientIP ->
	//    fingerprint ->
	//     recoverer ->
	//      logger ->
	//       traversal ->
	//        rateLimiter ->
	//         loadShedder ->
	//          acme ->
	//           httpsRedirector ->
	//            securityHeaders ->
	//             cors ->
	//              csrf ->
	//               Gzip ->
	//                reloadProtector ->
	//                 session ->
	//                  actual-handler

	// We have disabled Gzip for now, since it is about 2.5times slower than no-gzip for a 50MB sample response.
	// see: https://github.com/komuw/ong/issues/85

	// acme(wrappedHandler http.Handler, domain, acmeEmail, acmeDirectoryUrl string)
	// acmeEmail , acmeDirectoryUrl

	return trace(
		clientIP(
			fingerprint(
				recoverer(
					logger(
						pathTraversal(
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
																string(secretKey),
																domain,
																sessionCookieDuration,
																// TODO: use proper variable.
																func(r *http.Request) string { return r.RemoteAddr },
															),
															domain,
														),
														string(secretKey),
														domain,
														csrfTokenDuration,
													),
													allowedOrigins,
													allowedMethods,
													allowedHeaders,
													allowCredentials,
													corsCacheDuration,
												),
												domain,
											),
											httpsPort,
											domain,
										),
									),
									loadShedSamplingPeriod,
									loadShedMinSampleSize,
									loadShedBreachLatency,
								),
								rateLimit,
							),
						),
						l,
						rateShedSamplePercent,
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
func All(wrappedHandler http.Handler, o config.Opts) http.HandlerFunc {
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
func Get(wrappedHandler http.Handler, o config.Opts) http.HandlerFunc {
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
			w.Header().Add(allowHeader, "GET")
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
func Post(wrappedHandler http.Handler, o config.Opts) http.HandlerFunc {
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
			w.Header().Add(allowHeader, "POST")
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
func Head(wrappedHandler http.Handler, o config.Opts) http.HandlerFunc {
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
			w.Header().Add(allowHeader, "HEAD")
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
func Put(wrappedHandler http.Handler, o config.Opts) http.HandlerFunc {
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
			w.Header().Add(allowHeader, "PUT")
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
func Delete(wrappedHandler http.Handler, o config.Opts) http.HandlerFunc {
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
			w.Header().Add(allowHeader, "DELETE")
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

// httpRespCtrler represents the interface that has to be implemented for a
// responseWriter to satisfy [http.ResponseController]
//
// https://github.com/golang/go/blob/go1.21.0/src/net/http/responsecontroller.go#L42-L44
type httpRespCtrler interface {
	Unwrap() http.ResponseWriter
}
