// Package mx implements a HTTP request multiplexer.
// It is an internal package so that we can be able to add functionality to it that is used by ong, but cannot be called by third parties.
// Proper documentation for users should be added to github.com/komuw/ong/mux instead.
package mx

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strings"

	"github.com/komuw/ong/config"
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
	opt    config.Opts // needed by AddRoute
}

// String implements [fmt.Stringer]
func (m Muxer) String() string {
	return fmt.Sprintf(`Muxer{
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
func New(opt config.Opts, notFoundHandler http.Handler, routes ...Route) (Muxer, error) {
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

	// Try and detect conflicting routes.
	if err := detectConflict(m); err != nil {
		return Muxer{}, err
	}

	return m, nil
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

// AddRoute adds a new [Route] to an existing Mux.
// This is only expected to be used internally by ong.
// Users of ong should not use this method. Instead, pass all your routes when calling [New]
func (m Muxer) AddRoute(rt Route) error {
	// AddRoute should only be used internally by ong.
	m.addPattern(
		rt.method,
		rt.pattern,
		rt.originalHandler,
		middleware.All(rt.originalHandler, m.opt),
	)

	// Try and detect conflicting routes.
	if err := detectConflict(m); err != nil {
		return err
	}

	return nil
}

// Merge combines mxs into one. The resulting muxer uses the opts & notFoundHandler of muxer at index 0.
func Merge(mxs ...Muxer) (Muxer, error) {
	_len := len(mxs)

	if _len <= 0 {
		return Muxer{}, errors.New("ong/mux: no muxer")
	}
	if _len == 1 {
		return mxs[0], nil
	}

	m := mxs[0]
	for _, v := range mxs[1:] {
		m.router.routes = append(m.router.routes, v.router.routes...)
	}

	if err := detectConflict(m); err != nil {
		return m, err
	}

	return m, nil
}

// Param gets the path/url parameter from the specified Context.
func Param(ctx context.Context, param string) string {
	vStr, ok := ctx.Value(muxContextKey(param)).(string)
	if !ok {
		return ""
	}
	return vStr
}

// detectConflict returns an error with a diagnostic message when you try to add a route that would conflict with an already existing one.
//
// The error message looks like:
//
//	You are trying to add
//	  pattern: /post/:id/
//	  method: GET
//	  handler: github.com/myAPp/server/main.loginHandler - /home/server/main.go:351
//	However
//	  pattern: post/create
//	  method: GET
//	  handler: github.com/myAPp/server/main.logoutHandler - /home/server/main.go:345
//	already exists and would conflict.
//
// /
func detectConflict(m Muxer) error {
	for k := range m.router.routes {
		candidate := m.router.routes[k]
		pattern := candidate.pattern
		incomingSegments := pathSegments(pattern)

		for _, rt := range m.router.routes {
			if pattern == rt.pattern && (slices.Equal(candidate.segments, rt.segments)) && (getfunc(candidate.originalHandler) == getfunc(rt.originalHandler)) {
				continue
			}

			existingSegments := rt.segments
			sameLen := len(incomingSegments) == len(existingSegments)
			if !sameLen {
				// no conflict
				continue
			}

			errMsg := fmt.Errorf(`
You are trying to add
  pattern: %s
  method: %s
  handler: %v
However
  pattern: %s
  method: %s
  handler: %v
already exists and would conflict`,
				pattern,
				strings.ToUpper(candidate.method),
				getfunc(candidate.originalHandler),
				path.Join(rt.segments...),
				strings.ToUpper(rt.method),
				getfunc(rt.originalHandler),
			)

			if len(existingSegments) == 1 && existingSegments[0] == "*" && len(incomingSegments) > 0 {
				return errMsg
			}

			if pattern == rt.pattern {
				return errMsg
			}

			if strings.Contains(pattern, ":") && (incomingSegments[0] == existingSegments[0]) {
				return errMsg
			}

			if strings.Contains(rt.pattern, ":") && (incomingSegments[0] == existingSegments[0]) {
				return errMsg
			}
		}
	}

	return nil
}
