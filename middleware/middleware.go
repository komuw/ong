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
		}

		errMsg := fmt.Sprintf(msg, r.Method)
		http.Error(
			w,
			errMsg,
			http.StatusMethodNotAllowed,
		)
		return
	}
}
