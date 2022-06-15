package middleware

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/komuw/goweb/cookie"
	"github.com/rs/xid"
)

var errCsrfTokenNotFound = errors.New("csrf token not found")

type csrfContextKey string

const (
	csrfCtxKey   = csrfContextKey("csrfContextKey")
	defaultToken = ""
	cookieName   = "csrftoken"    // named after what django uses.
	cookieHeader = "X-Csrf-Token" // named after what fiber uses.
	cookieForm   = cookieName
)

// Csrf is a middleware that provides protection against Cross Site Request Forgeries.
func Csrf(wrappedHandler http.HandlerFunc, domain string) http.HandlerFunc {
	fromHeader := func(r *http.Request) string {
		return r.Header.Get(cookieHeader)
	}

	fromCookie := func(r *http.Request) string {
		if c, err := r.Cookie(cookieName); err != nil {
			return c.Value
		}
		return ""
	}

	fromForm := func(r *http.Request) string {
		if err := r.ParseForm(); err != nil {
			return ""
		}
		return r.Form.Get(cookieForm)
	}

	bloom := newBloom(10_000, 8)

	return func(w http.ResponseWriter, r *http.Request) {
		// - https://docs.djangoproject.com/en/4.0/ref/csrf/
		// - https://github.com/django/django/blob/4.0.5/django/middleware/csrf.py
		// - https://github.com/gofiber/fiber/blob/v2.34.1/middleware/csrf/csrf.go

		// 1. check http method.
		//     - if it is a 'safe' method like GET, try and get crsfToken from cookies.
		//     - if it is not a 'safe' method, try and get crsfToken from header/cookies/httpForm
		//        - take the found token and try to get it from memory store.
		//            - if not found in memory store, delete the cookie & return an error.

		ctx := r.Context()

		crsfToken := ""
		switch r.Method {
		// safe methods under rfc7231: https://datatracker.ietf.org/doc/html/rfc7231#section-4.2.1
		case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
			crsfToken = fromCookie(r)
		default:
			// For POST requests, we insist on a CSRF cookie, and in this way we can avoid all CSRF attacks, including login CSRF.
			crsfToken = fromCookie(r)
			if crsfToken == "" {
				crsfToken = fromHeader(r)
			}
			if crsfToken == "" {
				crsfToken = fromForm(r)
			}

			if crsfToken != "" && !bloom.get(crsfToken) {
				// bloom filter answers whether something is DEFINITELY NOT in the set.
				// so if the token is definitely not in memory store,
				// we should fail the request since it means that the server is not aware of such a token.
				cookie.Delete(w, cookieName, domain)
				http.Error(
					w,
					errCsrfTokenNotFound.Error(),
					http.StatusBadRequest,
				)
				return
			}
		}

		// 2. If crsfToken is still an empty string. generate it.
		if crsfToken == "" {
			crsfToken = xid.New().String()
		}

		// 3. save crsfToken in memory store.
		bloom.set(crsfToken)

		// 4. create cookie
		cookie.Set(
			w,
			cookieName,
			crsfToken,
			domain,
			// same max-age as what fiber uses. django seems to use one year.
			1*time.Hour,
			true,
		)

		// 5. set cookie header
		w.Header().Set(
			cookieHeader,
			crsfToken,
		)

		// 6. Vary header.

		// 7. store crsfToken in context
		r = r.WithContext(context.WithValue(ctx, csrfCtxKey, crsfToken))

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
	return defaultToken
}

// django:
// for safe methods like GET:
//   - call self._get_token(request) which gets token from cookies.
//   - if not available generate one.
//   - set cookie and header.
// for POST:
//    - call self._check_token()
//    - which calls self._get_token(request) which gets token from cookies.
//    - if not found raise REASON_NO_CSRF_COOKIE
//    - check form
//    - check header.
