package middleware

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/komuw/goweb/cookie"
	"github.com/rs/xid"
)

// Most of the code here is insipired by:
//   (a) https://github.com/gofiber/fiber whose license(MIT) can be found here:            https://github.com/gofiber/fiber/blob/v2.34.1/LICENSE
//   (b) https://github.com/django/django   whose license(BSD 3-Clause) can be found here: https://github.com/django/django/blob/4.0.5/LICENSE

var (
	errCsrfTokenNotFound = errors.New("csrf token not found/recognized")
	// csrfStore needs to be a global var so that different handlers that are decorated with the Csrf middleware can use same store.
	// Image if you had `Csrf(loginHandler, domain)` & `Csrf(cartCheckoutHandler, domain)`, if they didn't share a global store,
	// a customer navigating from login to checkout would get a errCsrfTokenNotFound error; which is not what we want.
	csrfStore = newStore() //nolint:gochecknoglobals
)

type csrfContextKey string

const (
	csrfCtxKey         = csrfContextKey("csrfContextKey")
	csrfDefaultToken   = ""
	csrfCookieName     = "csrftoken"    // named after what django uses.
	csrfHeader         = "X-Csrf-Token" // named after what fiber uses.
	csrfCookieForm     = csrfCookieName
	clientCookieHeader = "Cookie"
	varyHeader         = "Vary"

	// The memory store is reset(for memory efficiency) if either resetDuration OR maxRequestsToReset occurs.
	tokenMaxAge   = 1 * time.Hour // same max-age as what fiber uses. django seems to use one year.
	resetDuration = tokenMaxAge + (7 * time.Minute)
)

// Csrf is a middleware that provides protection against Cross Site Request Forgeries.
// If maxRequestsToReset <= 0, it is set to a high default value.
func Csrf(wrappedHandler http.HandlerFunc, domain string, maxRequestsToReset int32) http.HandlerFunc {
	start := time.Now()
	requestsServed := int32(0)
	if maxRequestsToReset <= 0 {
		maxRequestsToReset = 10_000_000
	}

	return func(w http.ResponseWriter, r *http.Request) {
		// - https://docs.djangoproject.com/en/4.0/ref/csrf/
		// - https://github.com/django/django/blob/4.0.5/django/middleware/csrf.py
		// - https://github.com/gofiber/fiber/blob/v2.34.1/middleware/csrf/csrf.go

		// 1. check http method.
		//     - if it is a 'safe' method like GET, try and get csrfToken from request.
		//     - if it is not a 'safe' method, try and get csrfToken from header/cookies/httpForm
		//        - take the found token and try to get it from memory store.
		//            - if not found in memory store, delete the cookie & return an error.

		ctx := r.Context()

		csrfToken := ""
		switch r.Method {
		// safe methods under rfc7231: https://datatracker.ietf.org/doc/html/rfc7231#section-4.2.1
		case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
			csrfToken = getToken(r)
		default:
			// For POST requests, we insist on a CSRF cookie, and in this way we can avoid all CSRF attacks, including login CSRF.
			csrfToken = getToken(r)

			if csrfToken == "" || !csrfStore.exists(csrfToken) {
				// we should fail the request since it means that the server is not aware of such a token.
				cookie.Delete(w, csrfCookieName, domain)
				http.Error(
					w,
					errCsrfTokenNotFound.Error(),
					http.StatusBadRequest,
				)
				return
			}
		}

		// 2. If csrfToken is still an empty string. generate it.
		if csrfToken == "" {
			csrfToken = xid.New().String()
		}

		// 3. create cookie
		cookie.Set(
			w,
			csrfCookieName,
			csrfToken,
			domain,
			tokenMaxAge,
			true,
		)

		// 4. set cookie header
		w.Header().Set(
			csrfHeader,
			csrfToken,
		)

		// 5. update Vary header.
		w.Header().Add(varyHeader, clientCookieHeader)

		// 6. store csrfToken in context
		r = r.WithContext(context.WithValue(ctx, csrfCtxKey, csrfToken))

		// 7. save csrfToken in memory store.
		csrfStore.set(csrfToken)

		// 8. reset memory to decrease its size.
		requestsServed = requestsServed + 1
		now := time.Now()
		diff := now.Sub(start)
		if (diff > resetDuration) || (requestsServed > maxRequestsToReset) {
			csrfStore.reset()
			start = now
			requestsServed = 0
		}

		wrappedHandler(w, r)
	}
}

// GetCsrfToken returns the csrf token was set for that particular request.
//
// usage:
//   func myHandler(w http.ResponseWriter, r *http.Request) {
//   	csrfToken := middleware.GetCsrfToken(r.Context())
//   	_ = csrfToken
//   }
func GetCsrfToken(c context.Context) string {
	v := c.Value(csrfCtxKey)
	if v != nil {
		s, ok := v.(string)
		if ok {
			return s
		}
	}
	return csrfDefaultToken
}

// getToken tries to fetch a csrf token from the incoming request r.
// It tries to fetch from cookies, headers, http-forms in that order.
func getToken(r *http.Request) string {
	fromCookie := func() string {
		c, err := r.Cookie(csrfCookieName)
		if err != nil {
			return ""
		}
		return c.Value
	}

	fromHeader := func() string {
		return r.Header.Get(csrfHeader)
	}

	fromForm := func() string {
		if err := r.ParseForm(); err != nil {
			return ""
		}
		return r.Form.Get(csrfCookieForm)
	}

	tok := fromCookie()
	if tok == "" {
		tok = fromHeader()
	}
	if tok == "" {
		tok = fromForm()
	}

	return tok
}

// store persists csrf tokens server-side, in-memory.
type store struct {
	mu sync.RWMutex // protects m
	m  map[string]struct{}
}

func newStore() *store {
	return &store{
		m: map[string]struct{}{},
	}
}

func (s *store) exists(csrfToken string) bool {
	s.mu.RLock()
	_, ok := s.m[csrfToken]
	s.mu.RUnlock()
	return ok
}

func (s *store) set(csrfToken string) {
	s.mu.Lock()
	s.m[csrfToken] = struct{}{}
	s.mu.Unlock()
}

func (s *store) reset() {
	s.mu.Lock()
	s.m = map[string]struct{}{}
	s.mu.Unlock()
}

// used in tests
func (s *store) _len() int {
	s.mu.RLock()
	l := len(s.m)
	s.mu.RUnlock()
	return l
}
