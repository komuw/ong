// Package mx implements a HTTP request multiplexer.
// It is an internal package so that we can be able to add functionality to it that is used by ong, but cannot be called by third parties.
// Proper documentation for users should be added to github.com/komuw/ong/mux instead.
package mx

import (
	"context"
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

		if err := m.addPattern(
			rt.method,
			rt.pattern,
			rt.originalHandler,
			mid(rt.originalHandler, opt),
		); err != nil {
			return Muxer{}, err
		}
	}

	return m, nil
}

func (m Muxer) addPattern(method, pattern string, originalHandler, wrappingHandler http.Handler) error {
	return m.router.handle(method, pattern, originalHandler, wrappingHandler)
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
	return m.addPattern(
		rt.method,
		rt.pattern,
		rt.originalHandler,
		middleware.All(rt.originalHandler, m.opt),
	)
}

// Merge combines mxs into m. The resulting muxer uses the opts & notFoundHandler of m.
func (m Muxer) Merge(mxs []Muxer) (Muxer, error) {
	if len(mxs) < 1 {
		return m, nil
	}

	// TODO: detect conflicts.
	for _, v := range mxs {
		m.router.routes = append(m.router.routes, v.router.routes...)
	}

	// TODO: merge this logic with `router.detectConflict`
	for k := range m.router.routes {
		incoming := m.router.routes[k] // TODO: rename this to candidate.
		pattern := incoming.pattern
		incomingSegments := pathSegments(pattern)

		for _, rt := range m.router.routes {
			if pattern == rt.pattern && (slices.Equal(incoming.segments, rt.segments)) && (getfunc(incoming.originalHandler) == getfunc(rt.originalHandler)) {
				// TODO: this is bad, we should not include the one for incoming.
				// Is this enough??
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
				strings.ToUpper(incoming.method),
				getfunc(incoming.originalHandler),
				path.Join(rt.segments...),
				strings.ToUpper(rt.method),
				getfunc(rt.originalHandler),
			)

			if len(existingSegments) == 1 && existingSegments[0] == "*" && len(incomingSegments) > 0 {
				return m, errMsg
			}

			if pattern == rt.pattern {
				return m, errMsg
			}

			if strings.Contains(pattern, ":") && (incomingSegments[0] == existingSegments[0]) {
				return m, errMsg
			}

			if strings.Contains(rt.pattern, ":") && (incomingSegments[0] == existingSegments[0]) {
				return m, errMsg
			}
		}
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
