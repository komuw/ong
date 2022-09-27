// TODO: docs.
package mux

import (
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

// Mux implements http.Handler
// Use [NewMux] to get a valid Mux.
type Mux struct {
	l      log.Logger
	router *Router // some router
}

func NewMux(l log.Logger, opt middleware.Opts, rts Routes) Mux {
	m := Mux{
		l:      l,
		router: NewRouter(),
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
func (m Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.router.ServeHTTP(w, r)
}

func (m Mux) addPattern(method, pattern string, handler http.HandlerFunc) {
	m.router.Handle(method, pattern, handler)
}
