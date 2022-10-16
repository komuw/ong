package middleware

import (
	"net/http"
	"time"

	"github.com/komuw/ong/sess"
)

// TODO: doc comment
// TODO: move to middleware/
// TODO: should this middleware should take some options(like cookie max-age) as arguments??
func Session(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	// TODO: make this variables
	secretKey := "secretKey"
	domain := "localhost"
	mAge := 2 * time.Hour

	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Read from cookies and check for session cookie.
		// 2. get that cookie and save it to r.context
		r = sess.Initialise(r, secretKey)

		// 1. Save session cookie to response.
		defer sess.Save(r, w, domain, mAge, secretKey)

		wrappedHandler(w, r)
	}
}
