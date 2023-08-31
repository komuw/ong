// Package mx implements a HTTP request multiplexer.
// It is an internal package so that we can be able to add functionality to it that is used by ong, but cannot be called by third parties.
// Proper documentation for users should be added to github.com/komuw/ong/mux instead.
package mx

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/komuw/ong/internal/acme"
	"github.com/komuw/ong/middleware"
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

// Muxer is a HTTP request multiplexer.
type Muxer struct {
	router *router
	opt    middleware.Opts // needed by AddRoute
}

// String implements [fmt.Stringer]
func (m Muxer) String() string {
	return fmt.Sprintf(`Opts{
  router: %v
  opt: %s
}`,
		m.router,
		m.opt,
	)
}

// GoString implements [fmt.GoStringer]
func (m Muxer) GoString() string {
	return m.String()
}

// New returns a HTTP request multiplexer that has the paths in routes.
func New(opt middleware.Opts, notFoundHandler http.Handler, routes ...Route) Muxer {
	m := Muxer{
		router: newRouter(notFoundHandler),
		opt:    opt,
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

	// TODO: remove this.
	{
		// Support for acme certificate manager needs to be added in three places:
		// (a) In http middlewares.
		// (b) In http server.
		// (c) In http multiplexer.
		const acmeChallengeURI = "/.well-known/acme-challenge/:token"
		acmeHandler := acme.Handler(m)
		m.addPattern(
			MethodAll,
			acmeChallengeURI,
			acmeHandler,
			middleware.All(acmeHandler, opt),
		)
	}

	return m
}

// TODO: maybe this should return an error??
// AddRoute adds a new [Route] to an existing Mux.
// This is only expected to be used internally by ong.
// Users of ong should not use this method. Instead, pass all your routes when calling [New]
func (m Muxer) AddRoute(rt Route) {
	// The m.AddRoute method should only be used internally by ong.
	m.addPattern(
		rt.method,
		rt.pattern,
		rt.originalHandler,
		middleware.All(rt.originalHandler, m.opt),
	)
}

func (m Muxer) addPattern(method, pattern string, originalHandler, wrappingHandler http.Handler) {
	m.router.handle(method, pattern, originalHandler, wrappingHandler)
}

// ServeHTTP implements a http.Handler
func (m Muxer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.router.serveHTTP(w, r)
}

// Resolve resolves a URL path to its corresponding [Route] and hence http handler.
func (m Muxer) Resolve(path string) Route {
	zero := Route{}

	u, err := url.Parse(path)
	if err != nil {
		return zero
	}

	{
		// todo: unify this logic with that found in `router.serveHTTP`
		segs := pathSegments(u.Path)
		for _, rt := range m.router.routes {
			if _, ok := rt.match(context.Background(), segs); ok {
				return rt
			}
		}
	}

	return zero
}

// Param gets the path/url parameter from the specified Context.
func Param(ctx context.Context, param string) string {
	vStr, ok := ctx.Value(muxContextKey(param)).(string)
	if !ok {
		return ""
	}
	return vStr
}
