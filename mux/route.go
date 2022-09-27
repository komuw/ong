package mux

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"runtime"
	"strings"
)

// Most of the code here is insipired by(or taken from):
//   (a) https://github.com/matryer/way whose license(MIT) can be found here: https://github.com/matryer/way/blob/9632d0c407b008073d19d0c4da1e0fc3e9477508/LICENSE

// muxContextKey is the context key type for storing path parameters in context.Context.
type muxContextKey string

type route struct {
	method  string
	pattern string
	segs    []string
	handler http.HandlerFunc
}

func (r route) String() string {
	return fmt.Sprintf("route{method: %s, pattern: %s, segs: %s}", r.method, r.pattern, r.segs)
}

func (r route) match(ctx context.Context, segs []string) (context.Context, bool) {
	if len(segs) > len(r.segs) {
		return nil, false
	}
	for i, seg := range r.segs {
		if i > len(segs)-1 {
			return nil, false
		}
		isParam := false
		if strings.HasPrefix(seg, ":") {
			isParam = true
			seg = strings.TrimPrefix(seg, ":")
		}
		if !isParam { // verbatim check
			if seg != segs[i] {
				return nil, false
			}
		}
		if isParam {
			ctx = context.WithValue(ctx, muxContextKey(seg), segs[i])
		}
	}
	return ctx, true
}

// router routes HTTP requests.
type router struct {
	routes []route
	// notFoundHandler is the http.Handler to call when no routes
	// match. By default uses http.NotFoundHandler().
	notFoundHandler http.Handler
}

// NewRouter makes a new Router.
func newRouter() *router {
	return &router{
		// TODO: add ability for someone to pass in a notFound handler.
		// If they pass in `nil` we default to `http.NotFoundHandler()`
		notFoundHandler: http.NotFoundHandler(),
	}
}

func pathSegments(p string) []string {
	return strings.Split(strings.Trim(p, "/"), "/")
}

// handle adds a handler with the specified method and pattern.
// Pattern can contain path segments such as: /item/:id which is
// accessible via the Param function.
func (r *router) handle(method, pattern string, handler http.HandlerFunc) {
	if !strings.HasSuffix(pattern, "/") {
		// this will make the mux send requests for;
		//   - localhost:80/check
		//   - localhost:80/check/
		// to the same handler.
		pattern = pattern + "/"
	}
	if !strings.HasPrefix(pattern, "/") {
		pattern = "/" + pattern
	}

	// Try and detect conflict before adding a new route.
	r.detectConflict(method, pattern, handler)

	rt := route{
		method:  strings.ToLower(method),
		pattern: pattern,
		segs:    pathSegments(pattern),
		handler: handler,
	}
	r.routes = append(r.routes, rt)
}

// serveHTTP routes the incoming http.Request based on method and path extracting path parameters as it goes.
func (r *router) serveHTTP(w http.ResponseWriter, req *http.Request) {
	segs := pathSegments(req.URL.Path)
	for _, route := range r.routes {
		if ctx, ok := route.match(req.Context(), segs); ok {
			route.handler.ServeHTTP(w, req.WithContext(ctx))
			return
		}
	}
	r.notFoundHandler.ServeHTTP(w, req)
}

// detectConflict panics with a diagnostic message when you try to add a route that would conflict with an already existing one.
//
// The panic message looks like:
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
func (r *router) detectConflict(method, pattern string, handler http.Handler) {
	// Conflicting routes are a bad thing.
	// They can be a source of bugs and confusion.
	// see: https://www.alexedwards.net/blog/which-go-router-should-i-use

	incomingSegments := pathSegments(pattern)
	for _, route := range r.routes {
		existingSegments := route.segs
		sameLen := len(incomingSegments) == len(existingSegments)
		if !sameLen {
			// no conflict
			break
		}

		panicMsg := fmt.Sprintf(`

You are trying to add
  pattern: %s
  method: %s
  handler: %v
However
  pattern: %s
  method: %s
  handler: %v
already exists and would conflict.`,
			pattern,
			strings.ToUpper(method),
			getfunc(handler),
			strings.Join(route.segs, "/"),
			strings.ToUpper(route.method),
			getfunc(route.handler),
		)

		for _, v := range incomingSegments {
			if strings.Contains(v, ":") {
				panic(panicMsg)
			}
		}
		for _, v := range existingSegments {
			if strings.Contains(v, ":") {
				panic(panicMsg)
			}
		}

		if pattern == route.pattern {
			panic(panicMsg)
		}
	}
}

func getfunc(handler interface{}) string {
	fn := runtime.FuncForPC(reflect.ValueOf(handler).Pointer())
	file, line := fn.FileLine(fn.Entry())
	return fmt.Sprintf("%s - %s:%d", fn.Name(), file, line)
}
