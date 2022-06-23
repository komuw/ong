// Package middleware provides helpful functions that implement some common functionalities in http servers.
// A middleware is a func that returns a http.HandlerFunc
package middleware

import (
	"fmt"
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
