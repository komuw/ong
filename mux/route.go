package mux

import (
	"net/http"

	"github.com/komuw/ong/internal/mx"
)

// Route represents the pattern & http method that will be served by a particular http handler.
//
// Use [NewRoute] to get a valid Route.
type Route = mx.Route

// NewRoute creates a new Route.
//
// It panics if handler has already been wrapped with ong/middleware
func NewRoute(
	pattern string,
	method string,
	handler http.Handler,
) Route {
	return mx.NewRoute(pattern, method, handler)
}
