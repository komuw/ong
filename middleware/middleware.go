// Package middleware provides helpful functions that implement some common functionalities in http servers.
// A middleware is a func that returns a http.HandlerFunc
package middleware

import (
	"fmt"
	"io"
	"net/http"
)

type opts struct {
	domain             string
	maxRequestsToReset int
	allowedOrigins     []string
	allowedMethods     []string
	allowedHeaders     []string
	logOutput          io.Writer
}

// allDefaultMiddlewares is a middleware that bundles all the default/core middlewares into one.
//
// usage:
//   allDefaultMiddlewares(wh, opts{"example.com", -1, nil, nil, nil, os.Stdout})
//
func allDefaultMiddlewares(
	wrappedHandler http.HandlerFunc,
	o opts,
) http.HandlerFunc {
	domain := o.domain
	maxRequestsToReset := o.maxRequestsToReset
	allowedOrigins := o.allowedOrigins
	allowedMethods := o.allowedOrigins
	allowedHeaders := o.allowedHeaders
	logOutput := o.logOutput

	// TODO: add load-shedding & ratelimiting.
	//   Those will probably come in between log & security.

	// The way the middlewares are layered is:
	// 1. panic on the outer since we want it to watch all other middlewares.
	// 2. log since we would like to get logs as early in the lifecycle as possible.
	// 3. security since we want some minimum level of security.
	// 4. cors since we might get pre-flight requests and we don't want those to go through all the middlewares for performance reasons.
	// 5. csrf since this one is a bit more involved perf-wise.
	// 6. gzip since it is very involved perf-wise.
	//
	// user -> panic -> log -> security -> cors -> csrf -> gzip -> actual-handler

	return Panic(
		Log(
			Security(
				Cors(
					Csrf(
						Gzip(
							wrappedHandler,
						),
						domain,
						maxRequestsToReset,
					),
					allowedOrigins,
					allowedMethods,
					allowedHeaders,
				),
				domain,
			),
			domain,
			logOutput,
		),
		logOutput,
	)
}

// Get is a middleware that only allows http GET requests and http OPTIONS requests.
func Get(wrappedHandler http.HandlerFunc, o opts) http.HandlerFunc {
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
func Post(wrappedHandler http.HandlerFunc, o opts) http.HandlerFunc {
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
func Head(wrappedHandler http.HandlerFunc, o opts) http.HandlerFunc {
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
func Put(wrappedHandler http.HandlerFunc, o opts) http.HandlerFunc {
	return allDefaultMiddlewares(
		head(wrappedHandler),
		o,
	)
}

func put(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	msg := "http method: %s not allowed. only allows http PUT"
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			errMsg := fmt.Sprintf(msg, r.Method)
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
func Delete(wrappedHandler http.HandlerFunc, o opts) http.HandlerFunc {
	return allDefaultMiddlewares(
		head(wrappedHandler),
		o,
	)
}

func delete(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	msg := "http method: %s not allowed. only allows http DELETE"
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			errMsg := fmt.Sprintf(msg, r.Method)
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
