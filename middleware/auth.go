package middleware

import (
	"crypto/subtle"
	"net/http"
)

// BasicAuth is a middleware that protects wrappedHandler using basic authentication.
func BasicAuth(wrappedHandler http.HandlerFunc, user, passwd string) http.HandlerFunc {
	const realm = "enter username and password"

	e := func(w http.ResponseWriter) {
		w.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	return func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if u == "" || p == "" || !ok {
			e(w)
			return
		}

		if subtle.ConstantTimeCompare([]byte(u), []byte(user)) != 1 {
			e(w)
			return
		}

		if subtle.ConstantTimeCompare([]byte(p), []byte(passwd)) != 1 {
			e(w)
			return
		}

		wrappedHandler(w, r)
	}
}
