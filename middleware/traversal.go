package middleware

import (
	"net/http"
)

// pathTraversal is a middleware that tries to prevent path pathTraversal attacks.
func pathTraversal(wrappedHandler http.Handler) http.HandlerFunc {
	/*
		- https://github.com/komuw/ong/issues/381
		- https://github.com/golang/go/issues/54385
	*/
	return func(w http.ResponseWriter, r *http.Request) {
		r.URL = r.URL.JoinPath()

		wrappedHandler.ServeHTTP(w, r)
	}
}
