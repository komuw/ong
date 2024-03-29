package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/komuw/ong/internal/key"
)

// BasicAuth is a middleware that protects wrappedHandler using basic authentication.
func BasicAuth(wrappedHandler http.Handler, user, passwd, hint string) http.HandlerFunc {
	if err := key.IsSecure(passwd); err != nil {
		panic(err)
	}

	// See: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/WWW-Authenticate
	realm := `enter username and password: ` + hint
	e := func(w http.ResponseWriter) {
		errMsg := `Basic realm=` + realm
		w.Header().Set("WWW-Authenticate", errMsg)
		w.Header().Set(ongMiddlewareErrorHeader, errMsg)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
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

		wrappedHandler.ServeHTTP(w, r)
	}
}
