package middleware

import (
	"crypto/subtle"
	"fmt"
	"net/http"
)

const minPasswdSize = 16

// BasicAuth is a middleware that protects wrappedHandler using basic authentication.
func BasicAuth(wrappedHandler http.Handler, user, passwd string) http.Handler {
	const realm = "enter username and password"

	if len(passwd) < minPasswdSize {
		panic(fmt.Sprintf("passwd cannot be less than %d in size.", minPasswdSize))
	}

	e := func(w http.ResponseWriter) {
		errMsg := `Basic realm="` + realm + `"`
		w.Header().Set("WWW-Authenticate", errMsg)
		w.Header().Set(ongMiddlewareErrorHeader, errMsg)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}

	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
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

			wrappedHandler.ServeHTTP(w, r)
		},
	)
}
