package middleware

import (
	"fmt"
	"net/http"
)

// pathTraversal is a middleware that tries to prevent path pathTraversal attacks.
func pathTraversal(wrappedHandler http.Handler) http.HandlerFunc {
	/*
		- https://github.com/komuw/ong/issues/381
		- https://github.com/golang/go/issues/54385
	*/
	return func(w http.ResponseWriter, r *http.Request) {
		// TODO: integration tests
		r.URL = r.URL.JoinPath()
		fmt.Println("\t trace: r.URL.Path: ", r.Method, r.URL.Path)         // TODO:
		fmt.Println("\t trace: r.URL.String(): ", r.Method, r.URL.String()) // TODO:

		wrappedHandler.ServeHTTP(w, r)
	}
}
