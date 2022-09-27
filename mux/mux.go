// Package mux implements a HTTP request multiplexer.
package mux

import (
	"context"
	"net/http"

	"github.com/komuw/ong/log"
	"github.com/komuw/ong/middleware"
)

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

// Routes is a list of all the route for an application.
type Routes []route

// NewRoute creates a new route.
func NewRoute(
	pattern string,
	method string,
	handler http.HandlerFunc,
) route {
	return route{
		method:  method,
		pattern: pattern,
		segs:    pathSegments(pattern),
		handler: handler,
	}
}

// Mux is a HTTP request multiplexer.
// It matches the URL of each incoming request against a list of registered
// patterns and calls the handler for the pattern that most closely matches the URL.
// It implements http.Handler
//
// Use [New] to get a valid Mux.
type Mux struct {
	l      log.Logger
	router *router // some router
}

// New return a HTTP request multiplexer that has the routes/paths in rts.
func New(l log.Logger, opt middleware.Opts, rts Routes) Mux {
	m := Mux{
		l:      l,
		router: newRouter(),
	}

	mid := middleware.All //nolint:ineffassign
	for _, rt := range rts {
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
			mid(rt.handler, opt),
		)
	}

	return m
}

// ServeHTTP implements a http.Handler
// It routes incoming http requests based on method and path extracting path parameters as it goes.
func (m Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.router.serveHTTP(w, r)
}

func (m Mux) addPattern(method, pattern string, handler http.HandlerFunc) {
	m.router.handle(method, pattern, handler)
}

// Param gets the path parameter from the specified Context.
// Returns an empty string if the parameter was not found.
func Param(ctx context.Context, param string) string {
	vStr, ok := ctx.Value(muxContextKey(param)).(string)
	if !ok {
		return ""
	}
	return vStr
}
