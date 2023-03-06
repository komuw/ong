// Package mux implements a HTTP request multiplexer.
package mux

import (
	"context"
	"net/http"
	"strings"

	"github.com/komuw/ong/middleware"

	"golang.org/x/exp/slog"
)

// Common HTTP methods.
const (
	MethodAll     = "ALL"
	MethodGet     = http.MethodGet
	MethodHead    = http.MethodHead
	MethodPost    = http.MethodPost
	MethodPut     = http.MethodPut
	MethodPatch   = http.MethodPatch
	MethodDelete  = http.MethodDelete
	MethodConnect = http.MethodConnect
	MethodOptions = http.MethodOptions
	MethodTrace   = http.MethodTrace
)

// NewRoute creates a new Route.
//
// It panics if handler has already been wrapped with ong/middleware
func NewRoute(
	pattern string,
	method string,
	handler http.HandlerFunc,
) Route {
	h := getfunc(handler)
	if strings.Contains(h, "ong/middleware/") &&
		!strings.Contains(h, "ong/middleware.BasicAuth") {
		// BasicAuth is allowed.
		panic("the handler should not be wrapped with ong middleware")
	}

	return Route{
		method:          method,
		pattern:         pattern,
		segs:            pathSegments(pattern),
		originalHandler: handler,
	}
}

// Mux is a HTTP request multiplexer.
//
// It matches the URL of each incoming request against a list of registered
// patterns and calls the handler for the pattern that most closely matches the URL.
// It implements http.Handler
//
// Use [New] to get a valid Mux.
type Mux struct {
	l      *slog.Logger
	router *router // some router
}

// New return a HTTP request multiplexer that has the paths in routes.
//
// notFoundHandler is the handler that will be used if a url is not found.
// If it is nil, [http.NotFound] is used instead.
//
// All the paths of an application should be added as part of the routes slice argument.
// Typically, an application should only have one Mux.
//
// It panics with a helpful error message if it detects conflicting routes.
func New(l *slog.Logger, opt middleware.Opts, notFoundHandler http.HandlerFunc, routes ...Route) Mux {
	m := Mux{
		l:      l,
		router: newRouter(notFoundHandler),
	}

	mid := middleware.All //nolint:ineffassign
	for _, rt := range routes {
		switch rt.method {
		case MethodAll:
			mid = middleware.All
		case MethodGet:
			mid = middleware.Get
		case MethodPost:
			mid = middleware.Post
		case MethodHead:
			mid = middleware.Head
		case MethodPut:
			mid = middleware.Put
		case MethodDelete:
			mid = middleware.Delete
		default:
			mid = middleware.All
		}

		m.addPattern(
			rt.method,
			rt.pattern,
			rt.originalHandler,
			mid(rt.originalHandler, opt),
		)
	}

	return m
}

// ServeHTTP implements a http.Handler
//
// It routes incoming http requests based on method and path extracting path parameters as it goes.
func (m Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.router.serveHTTP(w, r)
}

func (m Mux) addPattern(method, pattern string, originalHandler, wrappedHandler http.HandlerFunc) {
	m.router.handle(method, pattern, originalHandler, wrappedHandler)
}

// Param gets the path/url parameter from the specified Context.
// It returns an empty string if the parameter was not found.
func Param(ctx context.Context, param string) string {
	vStr, ok := ctx.Value(muxContextKey(param)).(string)
	if !ok {
		return ""
	}
	return vStr
}
