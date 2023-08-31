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
type Muxer = mx.Muxer

// New returns a HTTP request multiplexer that has the paths in routes.
//
// notFoundHandler is the handler that will be used if a url is not found.
// If it is nil, [http.NotFound] is used instead.
//
// All the paths of an application should be added as part of the routes slice argument.
// Typically, an application should only have one mx.
//
// It panics with a helpful error message if it detects conflicting routes.
func New(opt middleware.Opts, notFoundHandler http.Handler, routes ...mx.Route) Muxer {
	return mx.New(opt, notFoundHandler, routes...)
}

// Param gets the path/url parameter from the specified Context.
// It returns an empty string if the parameter was not found.
func Param(ctx context.Context, param string) string {
	return mx.Param(ctx, param)
}
