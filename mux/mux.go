// Package mux implements a HTTP request multiplexer.
package mux

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"runtime"
	"strings"

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

// NewRoute creates a new Route.
//
// It panics if handler has already been wrapped with ong/middleware
func NewRoute(
	pattern string,
	method string,
	handler http.Handler,
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
		segments:        pathSegments(pattern),
		originalHandler: handler,
	}
}

// Muxer is a HTTP request multiplexer.
//
// It matches the URL of each incoming request against a list of registered
// patterns and calls the handler for the pattern that most closely matches the URL.
// It implements http.Handler
//
// Use [New] to get a valid Muxer.
type Muxer struct {
	router *router         // some router
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
//
// notFoundHandler is the handler that will be used if a url is not found.
// If it is nil, [http.NotFound] is used instead.
//
// All the paths of an application should be added as part of the routes slice argument.
// Typically, an application should only have one Mux.
//
// It panics with a helpful error message if it detects conflicting routes.
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

	// TODO: rmeove this.
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
	_, file, _, _ := runtime.Caller(1)
	if strings.Contains(file, "/ong/server/") || strings.Contains(file, "/ong/mux/") {
		// The m.AddRoute method should only be used internally by ong.
		m.addPattern(
			rt.method,
			rt.pattern,
			rt.originalHandler,
			middleware.All(rt.originalHandler, m.opt),
		)
	}
}

func (m Muxer) addPattern(method, pattern string, originalHandler, wrappingHandler http.Handler) {
	m.router.handle(method, pattern, originalHandler, wrappingHandler)
}

// ServeHTTP implements a http.Handler
//
// It routes incoming http requests based on method and path extracting path parameters as it goes.
func (m Muxer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.router.serveHTTP(w, r)
}

// Resolve resolves a URL path to its corresponding [Route] and hence http handler.
// If no corresponding route/handler is found, a zero value [Route] is returned.
//
// It is not intended for use in production settings, it is more of a dev/debugging tool.
// It is inspired by django's [resolve] url utility.
//
// [resolve]: https://docs.djangoproject.com/en/4.2/ref/urlresolvers/#django.urls.resolve
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
// It returns an empty string if the parameter was not found.
func Param(ctx context.Context, param string) string {
	vStr, ok := ctx.Value(muxContextKey(param)).(string)
	if !ok {
		return ""
	}
	return vStr
}
