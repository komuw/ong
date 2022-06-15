package middleware

import (
	"net/http"
)

const (
	cookieName   = "csrftoken"    // named after what django uses.
	cookieHeader = "X-Csrf-Token" // named after what fiber uses.
	cookieForm   = cookieName
)

// Csrf is a middleware that provides protection against Cross Site Request Forgeries.
func Csrf(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	fromHeader := func(r *http.Request) string {
		return r.Header.Get(cookieHeader)
	}

	fromCookie := func(r *http.Request) string {
		if c, err := r.Cookie(cookieName); err != nil {
			return c.Value
		}
		return ""
	}

	return func(w http.ResponseWriter, r *http.Request) {
		// - https://docs.djangoproject.com/en/4.0/ref/csrf/
		// - https://github.com/django/django/blob/4.0.5/django/middleware/csrf.py
		// - https://github.com/gofiber/fiber/blob/v2.34.1/middleware/csrf/csrf.go

		// 1. check http method.
		//     - if it is a 'safe' method like GET, try and get crsfToken from cookies.
		//     - if it is not a 'safe' method, try and get crsfToken from header/cookies/httpForm
		//        - take the found token and try to get it from memory store.
		//            - if not found in memory store, delete the cookie & return an error.

		crsfToken := ""
		switch r.Method {
		// safe methods under rfc7231: https://datatracker.ietf.org/doc/html/rfc7231#section-4.2.1
		case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
			crsfToken = fromCookie(r)
		default:
			crsfToken = fromHeader(r)
			if crsfToken == "" {
				crsfToken = fromCookie(r)
			}

			if crsfToken != "" {
			}
		}

		// 2. If crsfToken is still an empty string. generate it.

		// 3. save crsfToken in memory store.

		// 4. create cookie

		// 5. Vary header.

		// 6. store crsfToken in context

		wrappedHandler(w, r)
	}
}
