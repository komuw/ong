// Package middleware provides helpful functions that implement some common functionalities in http servers.
// A middleware is a func that returns a http.HandlerFunc
package middleware

import (
	"fmt"
	"io"
	"net/http"
)

// Get is a middleware that only allows http GET requests and http OPTIONS requests.
func Get(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	msg := "http method: %s not allowed. only allows http GET/OPTIONS"
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions || r.Method == http.MethodGet {
			// http OPTIONS is allowed because it is used for CORS(as a preflight request signal.)
			wrappedHandler(w, r)
		} else {
			errMsg := fmt.Sprintf(msg, r.Method)
			http.Error(
				w,
				errMsg,
				http.StatusMethodNotAllowed,
			)
			return
		}
	}
}

// Post is a middleware that only allows http POST requests and http OPTIONS requests.
func Post(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	msg := "http method: %s not allowed. only allows http POST/OPTIONS"
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions || r.Method == http.MethodPost {
			// http OPTIONS is allowed because it is used for CORS(as a preflight request signal.)
			wrappedHandler(w, r)
		} else {
			errMsg := fmt.Sprintf(msg, r.Method)
			http.Error(
				w,
				errMsg,
				http.StatusMethodNotAllowed,
			)
			return
		}
	}
}

// Head is a middleware that only allows http HEAD requests and http OPTIONS requests.
func Head(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	msg := "http method: %s not allowed. only allows http HEAD/OPTIONS"
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions || r.Method == http.MethodHead {
			// http OPTIONS is allowed because it is used for CORS(as a preflight request signal.)
			wrappedHandler(w, r)
		} else {
			errMsg := fmt.Sprintf(msg, r.Method)
			http.Error(
				w,
				errMsg,
				http.StatusMethodNotAllowed,
			)
			return
		}
	}
}

// Put is a middleware that only allows http PUT requests and http OPTIONS requests.
func Put(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	msg := "http method: %s not allowed. only allows http PUT/OPTIONS"
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions || r.Method == http.MethodPut {
			// http OPTIONS is allowed because it is used for CORS(as a preflight request signal.)
			wrappedHandler(w, r)
		} else {
			errMsg := fmt.Sprintf(msg, r.Method)
			http.Error(
				w,
				errMsg,
				http.StatusMethodNotAllowed,
			)
			return
		}
	}
}

// Delete is a middleware that only allows http DELETE requests and http OPTIONS requests.
func Delete(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	msg := "http method: %s not allowed. only allows http DELETE/OPTIONS"
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions || r.Method == http.MethodDelete {
			// http OPTIONS is allowed because it is used for CORS(as a preflight request signal.)
			wrappedHandler(w, r)
		} else {
			errMsg := fmt.Sprintf(msg, r.Method)
			http.Error(
				w,
				errMsg,
				http.StatusMethodNotAllowed,
			)
			return
		}
	}
}

/*
  get(wh, "example.com", -1, nil, nil, nil, os.Stdout)
*/
func get(
	wrappedHandler http.HandlerFunc,
	domain string,
	maxRequestsToReset int,
	allowedOrigins []string,
	allowedMethods []string,
	allowedHeaders []string,
	logOutput io.Writer,
) {
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

	Panic(
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
					allowedOrigins, allowedMethods, allowedHeaders,
				),
				domain,
			),
			domain,
			logOutput,
		),
		logOutput,
	)
}
