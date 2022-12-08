// Package middleware provides helpful functions that implement some common functionalities in http servers.
// A middleware is a function that returns a [http.HandlerFunc]
package middleware

import (
	"fmt"
	"net/http"

	"github.com/komuw/ong/log"
)

// ongMiddlewareErrorHeader is a http header that is set by Ong
// whenever any of it's middlewares return an error.
// The Log & recoverer middleware will log the value of this header if it is set.
//
// An example, is when the Get middleware fails because it has been called with the wrong http method.
// Or when the Csrf middleware fails because a csrf token was not found for POST/DELETE/etc requests.
const ongMiddlewareErrorHeader = "Ong-Middleware-Error"

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
	l              log.Logger
}

// New returns a new Opts.
//
// domain is the domain name of your website.
// httpsPort is the tls port where http requests will be redirected to.
// allowedOrigins, allowedMethods, & allowedHeaders are used by the [Cors] middleware.
//
// The secretKey should be kept secret and should not be shared.
// If it becomes compromised, generate a new one and restart your application using the new one.
func New(
	domain string,
	httpsPort uint16,
	allowedOrigins []string,
	allowedMethods []string,
	allowedHeaders []string,
	secretKey string,
	l log.Logger,
) Opts {
	return Opts{
		domain:         domain,
		httpsPort:      httpsPort,
		allowedOrigins: allowedOrigins,
		allowedMethods: allowedMethods,
		allowedHeaders: allowedHeaders,
		secretKey:      secretKey,
		l:              l,
	}
}

// WithOpts returns a new Opts that has sensible defaults.
func WithOpts(domain string, httpsPort uint16, secretKey string, l log.Logger) Opts {
	return New(domain, httpsPort, nil, nil, nil, secretKey, l)
}

// allDefaultMiddlewares is a middleware that bundles all the default/core middlewares into one.
//
// example usage:
//
//	allDefaultMiddlewares(wh, opts{"example.com", -1, nil, nil, nil, os.Stdout})
func allDefaultMiddlewares(
	wrappedHandler http.HandlerFunc,
	o Opts,
) http.HandlerFunc {
	domain := o.domain
	httpsPort := o.httpsPort
	allowedOrigins := o.allowedOrigins
	allowedMethods := o.allowedOrigins
	allowedHeaders := o.allowedHeaders
	secretKey := o.secretKey
	l := o.l

	// The way the middlewares are layered is:
	// 1.  recoverer on the outer since we want it to watch all other middlewares.
	// 2.  Log since we would like to get logs as early in the lifecycle as possible.
	// 3.  RateLimiter since we want bad traffic to be filtered early.
	// 4.  LoadShedder for the same reason.
	// 5.  HttpsRedirector since it can be cpu intensive, thus should be behind the ratelimiter & loadshedder.
	// 6.  SecurityHeaders since we want some minimum level of security.
	// 7.  Cors since we might get pre-flight requests and we don't want those to go through all the middlewares for performance reasons.
	// 8.  Csrf since this one is a bit more involved perf-wise.
	// 9.  Gzip since it is very involved perf-wise.
	// 10. ReloadProtector, ideally I feel like it should come earlier but I'm yet to figure out where.
	// 11. Session since we want sessions to saved as soon as possible.
	//
	// user -> recoverer -> Log -> RateLimiter -> LoadShedder -> HttpsRedirector -> SecurityHeaders -> Cors -> Csrf -> Gzip -> ReloadProtector -> Session -> actual-handler

	// We have disabled Gzip for now, since it is about 2.5times slower than no-gzip for a 50MB sample response.
	// see: https://github.com/komuw/ong/issues/85

	return recoverer(
		Log(
			RateLimiter(
				LoadShedder(
					HttpsRedirector(
						SecurityHeaders(
							Cors(
								Csrf(
									ReloadProtector(
										Session(
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
			domain,
			l,
		),
		l,
	)
}

// All is a middleware that allows all http methods.
//
// It is composed of the [recoverer], [Log], [RateLimiter], [LoadShedder], [HttpsRedirector], [SecurityHeaders], [Cors], [Csrf], [ReloadProtector] & [Session] middleware.
// As such, it provides the features and functionalities of all those middlewares.
func All(wrappedHandler http.HandlerFunc, o Opts) http.HandlerFunc {
	return allDefaultMiddlewares(
		all(wrappedHandler),
		o,
	)
}

func all(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		wrappedHandler(w, r)
	}
}

// Get is a middleware that only allows http GET requests and http OPTIONS requests.
//
// It is composed of the [recoverer], [Log], [RateLimiter], [LoadShedder], [HttpsRedirector], [SecurityHeaders], [Cors], [Csrf], [ReloadProtector] & [Session] middleware.
// As such, it provides the features and functionalities of all those middlewares.
func Get(wrappedHandler http.HandlerFunc, o Opts) http.HandlerFunc {
	return allDefaultMiddlewares(
		get(wrappedHandler),
		o,
	)
}

func get(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	msg := "http method: %s not allowed. only allows http GET"
	return func(w http.ResponseWriter, r *http.Request) {
		// We do not need to allow `http.MethodOptions` here.
		// This is coz, the Cors middleware has already handled that for us and it comes before the Get middleware.
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

		wrappedHandler(w, r)
	}
}

// Post is a middleware that only allows http POST requests and http OPTIONS requests.
//
// It is composed of the [recoverer], [Log], [RateLimiter], [LoadShedder], [HttpsRedirector], [SecurityHeaders], [Cors], [Csrf], [ReloadProtector] & [Session] middleware.
// As such, it provides the features and functionalities of all those middlewares.
func Post(wrappedHandler http.HandlerFunc, o Opts) http.HandlerFunc {
	return allDefaultMiddlewares(
		post(wrappedHandler),
		o,
	)
}

func post(wrappedHandler http.HandlerFunc) http.HandlerFunc {
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

		wrappedHandler(w, r)
	}
}

// Head is a middleware that only allows http HEAD requests and http OPTIONS requests.
//
// It is composed of the [recoverer], [Log], [RateLimiter], [LoadShedder], [HttpsRedirector], [SecurityHeaders], [Cors], [Csrf], [ReloadProtector] & [Session] middleware.
// As such, it provides the features and functionalities of all those middlewares.
func Head(wrappedHandler http.HandlerFunc, o Opts) http.HandlerFunc {
	return allDefaultMiddlewares(
		head(wrappedHandler),
		o,
	)
}

func head(wrappedHandler http.HandlerFunc) http.HandlerFunc {
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

		wrappedHandler(w, r)
	}
}

// Put is a middleware that only allows http PUT requests and http OPTIONS requests.
//
// It is composed of the [recoverer], [Log], [RateLimiter], [LoadShedder], [HttpsRedirector], [SecurityHeaders], [Cors], [Csrf], [ReloadProtector] & [Session] middleware.
// As such, it provides the features and functionalities of all those middlewares.
func Put(wrappedHandler http.HandlerFunc, o Opts) http.HandlerFunc {
	return allDefaultMiddlewares(
		put(wrappedHandler),
		o,
	)
}

func put(wrappedHandler http.HandlerFunc) http.HandlerFunc {
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

		wrappedHandler(w, r)
	}
}

// Delete is a middleware that only allows http DELETE requests and http OPTIONS requests.
//
// It is composed of the [recoverer], [Log], [RateLimiter], [LoadShedder], [HttpsRedirector], [SecurityHeaders], [Cors], [Csrf], [ReloadProtector] & [Session] middleware.
// As such, it provides the features and functionalities of all those middlewares.
func Delete(wrappedHandler http.HandlerFunc, o Opts) http.HandlerFunc {
	return allDefaultMiddlewares(
		deleteH(wrappedHandler),
		o,
	)
}

// this is not called `delete` since that is a Go builtin func for deleting from maps.
func deleteH(wrappedHandler http.HandlerFunc) http.HandlerFunc {
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

		wrappedHandler(w, r)
	}
}
