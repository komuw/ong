package middleware

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/komuw/ong/id"

	"github.com/komuw/ong/cookie"
)

// Most of the code here is insipired by(or taken from):
//   (a) https://github.com/gofiber/fiber whose license(MIT) can be found here:            https://github.com/gofiber/fiber/blob/v2.34.1/LICENSE
//   (b) https://github.com/django/django   whose license(BSD 3-Clause) can be found here: https://github.com/django/django/blob/4.0.5/LICENSE

var (
	// errCsrfTokenNotFound is returned when a request using a non-safe http method
	// either does not supply a csrf token, or the supplied token is not recognized by the server.
	errCsrfTokenNotFound = errors.New("csrf token not found")
	// csrfStore needs to be a global var so that different handlers that are decorated with the Csrf middleware can use same store.
	// Image if you had `Csrf(loginHandler, domain)` & `Csrf(cartCheckoutHandler, domain)`, if they didn't share a global store,
	// a customer navigating from login to checkout would get a errCsrfTokenNotFound error; which is not what we want.
	csrfStore = newStore() //nolint:gochecknoglobals
)

type csrfContextKey string

const (
	csrfCtxKey               = csrfContextKey("csrfContextKey")
	csrfDefaultToken         = ""
	csrfCookieName           = "csrftoken"    // named after what django uses.
	csrfHeader               = "X-Csrf-Token" // named after what fiber uses.
	csrfCookieForm           = csrfCookieName
	clientCookieHeader       = "Cookie"
	varyHeader               = "Vary"
	authorizationHeader      = "Authorization"
	proxyAuthorizationHeader = "Proxy-Authorization"

	// gorilla/csrf; 12hrs
	// django: 1yr??
	// gofiber/fiber; 1hr
	tokenMaxAge = 12 * time.Hour
	// The memory store is reset(for memory efficiency) every resetDuration.
	resetDuration = tokenMaxAge + (7 * time.Minute)

	// django appears to use 32 random characters for its csrf token.
	// so does gorilla/csrf; https://github.com/gorilla/csrf/blob/v1.7.1/csrf.go#L13-L14
	csrfBytesTokenLength = 32
	// we call `id.Random()` with `csrfBytesTokenLength` and it returns a string of `csrfStringTokenlength`
	csrfStringTokenlength = 43
)

// Csrf is a middleware that provides protection against Cross Site Request Forgeries.
func Csrf(wrappedHandler http.HandlerFunc, domain string) http.HandlerFunc {
	start := time.Now()

	return func(w http.ResponseWriter, r *http.Request) {
		// - https://docs.djangoproject.com/en/4.0/ref/csrf/
		// - https://github.com/django/django/blob/4.0.5/django/middleware/csrf.py
		// - https://github.com/gofiber/fiber/blob/v2.34.1/middleware/csrf/csrf.go

		// 1. check http method.
		//     - if it is a 'safe' method like GET, try and get `actualToken` from request.
		//     - if it is not a 'safe' method, try and get `actualToken` from header/cookies/httpForm
		//        - take the found token and try to get it from memory store.
		//            - if not found in memory store, delete the cookie & return an error.

		ctx := r.Context()

		actualToken := ""
		switch r.Method {
		// safe methods under rfc7231: https://datatracker.ietf.org/doc/html/rfc7231#section-4.2.1
		case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
			actualToken = getToken(r)
		default:
			// For POST requests, we insist on a CSRF cookie, and in this way we can avoid all CSRF attacks, including login CSRF.
			actualToken = getToken(r)

			// if r.Header.Get(clientCookieHeader) == "" &&
			// 	r.Header.Get(authorizationHeader) == "" &&
			// 	r.Header.Get(proxyAuthorizationHeader) == "" {
			// 	break
			// }

			if !csrfStore.exists(actualToken) {
				// we should fail the request since it means that the server is not aware of such a token.
				cookie.Delete(w, csrfCookieName, domain)
				w.Header().Set(ongMiddlewareErrorHeader, errCsrfTokenNotFound.Error())
				http.Error(
					w,
					errCsrfTokenNotFound.Error(),
					http.StatusForbidden,
				)
				return
			}
		}

		// 2. If actualToken is still an empty string. generate it.
		if actualToken == "" {
			actualToken = id.Random(csrfBytesTokenLength)
		}

		/*
			We need to try and protect against BreachAttack[1]. See[2] for a refresher on how it works.
			The mitigations against the attack in order of effectiveness are:
			(a) Disabling HTTP compression
			(b) Separating secrets from user input
			(c) Randomizing secrets per request
			(d) Masking secrets (effectively randomizing by XORing with a random secret per request)
			(e) Protecting vulnerable pages with CSRF
			(f) Length hiding (by adding random number of bytes to the responses)
			(g) Rate-limiting the requests
			Most csrf implementation use (d). Here, we'll use (c)

			1. http://breachattack.com/
			2. https://security.stackexchange.com/a/172646
		*/
		breachAttackToken := id.Random(csrfBytesTokenLength)
		tokenToIssue := breachAttackToken + actualToken

		// 3. create cookie
		cookie.Set(
			w,
			csrfCookieName,
			tokenToIssue,
			domain,
			tokenMaxAge,
			true, // accessible to javascript
		)

		// 4. set cookie header
		w.Header().Set(
			csrfHeader,
			tokenToIssue,
		)

		// 5. update Vary header.
		w.Header().Add(varyHeader, clientCookieHeader)

		// 6. store tokenToIssue in context
		r = r.WithContext(context.WithValue(ctx, csrfCtxKey, tokenToIssue))

		// 7. save `actualToken` in memory store.
		csrfStore.set(actualToken)

		// 8. reset memory to decrease its size.
		now := time.Now()
		diff := now.Sub(start)
		if diff > resetDuration {
			csrfStore.reset()
			start = now
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
// It tries to fetch from cookies, http-forms, headers in that order.
func getToken(r *http.Request) (actualToken string) {
	fromCookie := func() string {
		c, err := r.Cookie(csrfCookieName)
		if err != nil {
			return ""
		}
		return c.Value
	}

	fromForm := func() string {
		return r.FormValue(csrfCookieForm) // calls ParseMultipartForm and ParseForm if necessary
	}

	fromHeader := func() string {
		return r.Header.Get(csrfHeader)
	}

	tok := fromCookie()
	if tok == "" {
		tok = fromForm()
	}
	if tok == "" {
		tok = fromHeader()
	}

	if len(tok) != (2 * csrfStringTokenlength) {
		// Request has presented a token that we probably didn't generate coz this library issues
		// a token that is of length (2 * csrfStringTokenlength).
		// So, set it to empty string so that a proper token will get generated.
		tok = ""
	} else {
		// In this lib, the token we issue to the user is `breachAttackToken + actualToken`
		// So here we retrieve the actual token
		tok = tok[csrfStringTokenlength:]
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

func (s *store) exists(actualToken string) bool {
	if len(actualToken) < 1 {
		return false
	}
	s.mu.RLock()
	_, ok := s.m[actualToken]
	s.mu.RUnlock()
	return ok
}

func (s *store) set(actualToken string) {
	s.mu.Lock()
	s.m[actualToken] = struct{}{}
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
