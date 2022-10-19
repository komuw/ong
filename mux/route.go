package mux

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"reflect"
	"runtime"
	"strings"
)

// Most of the code here is insipired by(or taken from):
//   (a) https://github.com/matryer/way whose license(MIT) can be found here: https://github.com/matryer/way/blob/9632d0c407b008073d19d0c4da1e0fc3e9477508/LICENSE

// muxContextKey is the context key type for storing path parameters in context.Context.
type muxContextKey string

// Route represents the pattern & http method that will be served by a particular http handler.
//
// Use [NewRoute] to get a valid Route.
type Route struct {
	method  string
	pattern string
	segs    []string
	handler http.HandlerFunc
}

func (r Route) String() string {
	return fmt.Sprintf(`
Route{
  method: %s,
  pattern: %s,
  segs: %s,
  handler: %s
}`, r.method, r.pattern, r.segs, getfunc(r.handler))
}

func (r Route) match(ctx context.Context, segs []string) (context.Context, bool) {
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
	routes []Route
	// notFoundHandler is the handler to call when no routes match.
	notFoundHandler http.HandlerFunc
}

// NewRouter makes a new Router.
func newRouter(notFoundHandler http.HandlerFunc) *router {
	if notFoundHandler == nil {
		notFoundHandler = http.NotFound
	}

	return &router{notFoundHandler: notFoundHandler}
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

	rt := Route{
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
	for _, rt := range r.routes {
		if ctx, ok := rt.match(req.Context(), segs); ok {
			rt.handler.ServeHTTP(w, req.WithContext(ctx))
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
	for _, rt := range r.routes {
		existingSegments := rt.segs
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
			path.Join(rt.segs...),
			strings.ToUpper(rt.method),
			getfunc(rt.handler),
		)

		if pattern == rt.pattern {
			panic(panicMsg)
		}

		if strings.Contains(pattern, ":") && (incomingSegments[0] == existingSegments[0]) {
			panic(panicMsg)
		}

		if strings.Contains(rt.pattern, ":") && (incomingSegments[0] == existingSegments[0]) {
			panic(panicMsg)
		}
	}
}

func getfunc(handler interface{}) string {
	fn := runtime.FuncForPC(reflect.ValueOf(handler).Pointer())
	file, line := fn.FileLine(fn.Entry())
	return fmt.Sprintf("%s - %s:%d", fn.Name(), file, line)
}
