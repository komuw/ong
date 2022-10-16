package middleware

import (
	"net/http"
	"time"

	"github.com/komuw/ong/sess"
)

const (
	// django uses a value of 2 weeks by default.
	// https://docs.djangoproject.com/en/4.1/ref/settings/#session-cookie-age
	sessionMaxAge = 14 * time.Hour
)

// TODO: doc comment
// TODO: move to middleware/
// TODO: should this middleware should take some options(like cookie max-age) as arguments??
func Session(wrappedHandler http.HandlerFunc, secretKey, domain string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Read from cookies and check for session cookie.
		// 2. Get that cookie and save it to r.context
		r = sess.Initialise(r, secretKey)

		// 3. Save session cookie to response.
		defer sess.Save(r, w, domain, sessionMaxAge, secretKey)

		wrappedHandler(w, r)
	}
}
