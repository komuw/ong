package mx

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"reflect"
	"runtime"
	"strings"
)

// Most of the code here is inspired by(or taken from):
//   (a) https://github.com/matryer/way whose license(MIT) can be found here: https://github.com/matryer/way/blob/9632d0c407b008073d19d0c4da1e0fc3e9477508/LICENSE

// muxContextKey is the context key type for storing path parameters in context.Context.
type muxContextKey string

// Route represents the pattern & http method that will be served by a particular http handler.
//
// Use [NewRoute] to get a valid Route.
type Route struct {
	method          string
	pattern         string
	segments        []string
	originalHandler http.Handler // This is only needed to enhance the debug/panic message when conflicting routes are detected.
	wrappingHandler http.Handler
}

func (r Route) String() string {
	originHandler := ""
	if r.originalHandler != nil {
		originHandler = getfunc(r.originalHandler)
	}
	wrappingHandler := ""
	if r.wrappingHandler != nil {
		wrappingHandler = getfunc(r.wrappingHandler)
	}

	return fmt.Sprintf(`
Route{
  method: %s,
  pattern: %s,
  segments: %s,
  originalHandler: %s,
  wrappingHandler: %s,
}`, r.method, r.pattern, r.segments, originHandler, wrappingHandler)
}

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

func (r Route) match(ctx context.Context, segs []string) (context.Context, bool) {
	if len(segs) > len(r.segments) {
		return nil, false
	}
	for i, seg := range r.segments {
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
	notFoundHandler http.Handler
}

// String implements [fmt.Stringer]
func (r router) String() string {
	return fmt.Sprintf(`router{
  routes: %v
  notFoundHandler: %v
}`,
		r.routes,
		r.notFoundHandler,
	)
}

// GoString implements [fmt.GoStringer]
func (r router) GoString() string {
	return r.String()
}

// NewRouter makes a new Router.
func newRouter(notFoundHandler http.Handler) *router {
	if notFoundHandler == nil {
		notFoundHandler = http.NotFoundHandler()
	}

	return &router{notFoundHandler: notFoundHandler}
}

func pathSegments(p string) []string {
	return strings.Split(strings.Trim(p, "/"), "/")
}

// handle adds a handler with the specified method and pattern.
// Pattern can contain path segments such as: /item/:id which is
// accessible via the Param function.
func (r *router) handle(method, pattern string, originalHandler, wrappingHandler http.Handler) error {
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
	if err := r.detectConflict(method, pattern, originalHandler); err != nil {
		return err
	}

	rt := Route{
		method:          strings.ToUpper(method),
		pattern:         pattern,
		segments:        pathSegments(pattern),
		originalHandler: originalHandler,
		wrappingHandler: wrappingHandler,
	}
	r.routes = append(r.routes, rt)

	return nil
}

// serveHTTP routes the incoming http.Request based on method and path extracting path parameters as it goes.
func (r *router) serveHTTP(w http.ResponseWriter, req *http.Request) {
	segs := pathSegments(req.URL.Path)
	for _, rt := range r.routes {
		if ctx, ok := rt.match(req.Context(), segs); ok {
			rt.wrappingHandler.ServeHTTP(w, req.WithContext(ctx))
			return
		}
	}

	r.notFoundHandler.ServeHTTP(w, req)
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
func (r *router) detectConflict(method, pattern string, originalHandler http.Handler) error {
	// Conflicting routes are a bad thing.
	// They can be a source of bugs and confusion.
	// see: https://www.alexedwards.net/blog/which-go-router-should-i-use

	incomingSegments := pathSegments(pattern)
	for _, rt := range r.routes {
		existingSegments := rt.segments
		sameLen := len(incomingSegments) == len(existingSegments)
		if !sameLen {
			// no conflict
			break
		}

		panicMsg := fmt.Errorf(`

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
			getfunc(originalHandler),
			path.Join(rt.segments...),
			strings.ToUpper(rt.method),
			getfunc(rt.originalHandler),
		)

		if pattern == rt.pattern {
			return panicMsg
		}

		if strings.Contains(pattern, ":") && (incomingSegments[0] == existingSegments[0]) {
			return panicMsg
		}

		if strings.Contains(rt.pattern, ":") && (incomingSegments[0] == existingSegments[0]) {
			return panicMsg
		}
	}

	return nil
}

func getfunc(handler http.Handler) string {
	fn := runtime.FuncForPC(reflect.ValueOf(handler).Pointer())
	file, line := fn.FileLine(fn.Entry())
	return fmt.Sprintf("%s - %s:%d", fn.Name(), file, line)
}
