// Package mux implements a HTTP request multiplexer.
package mux

import (
	"context"
	"net/http"

	"github.com/komuw/ong/internal/mx"
	"github.com/komuw/ong/middleware"
)

// Common HTTP methods.
const (
	MethodAll     = mx.MethodAll
	MethodGet     = mx.MethodGet
	MethodHead    = mx.MethodHead
	MethodPost    = mx.MethodPost
	MethodPut     = mx.MethodPut
	MethodPatch   = mx.MethodPatch
	MethodDelete  = mx.MethodDelete
	MethodConnect = mx.MethodConnect
	MethodOptions = mx.MethodOptions
	MethodTrace   = mx.MethodTrace
)

// Muxer is a HTTP request multiplexer.
//
// It matches the URL of each incoming request against a list of registered
// patterns and calls the handler for the pattern that most closely matches the URL.
// It implements http.Handler
//
// Use [New] to get a valid Muxer.
type Muxer struct {
	internalMux mx.Muxer
}

// New returns a HTTP request multiplexer that has the paths in routes.
//
// notFoundHandler is the handler that will be used if a url is not found.
// If it is nil, [http.NotFound] is used instead.
//
// All the paths of an application should be added as part of the routes slice argument.
// Typically, an application should only have one mux.
//
// It panics with a helpful error message if it detects conflicting routes.
func New(opt middleware.Opts, notFoundHandler http.Handler, routes ...Route) Muxer {
	m, err := mx.New(opt, notFoundHandler, routes...)
	if err != nil {
		panic(err)
	}
	return Muxer{internalMux: m}
}

// ServeHTTP implements a http.Handler
//
// It routes incoming http requests based on method and path extracting path parameters as it goes.
func (m Muxer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.internalMux.ServeHTTP(w, r)
}

// Resolve resolves a URL path to its corresponding [Route] and hence http handler.
// If no corresponding route/handler is found, a zero value [Route] is returned.
//
// It is not intended for use in production settings, it is more of a dev/debugging tool.
// It is inspired by django's [resolve] url utility.
//
// [resolve]: https://docs.djangoproject.com/en/4.2/ref/urlresolvers/#django.urls.resolve
func (m Muxer) Resolve(path string) Route {
	return m.internalMux.Resolve(path)
}

// TODO: test.
// Unwrap returns the underlying muxer.
// It is for internal use(ONLY) by ong. Users of ong should not need to call it.
func (m Muxer) Unwrap() mx.Muxer {
	return m.internalMux
}

// Param gets the path/url parameter from the specified Context.
// It returns an empty string if the parameter was not found.
func Param(ctx context.Context, param string) string {
	return mx.Param(ctx, param)
}
