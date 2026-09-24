package middleware

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"

	"github.com/komuw/ong/cry"
)

// BasicAuth is a middleware that protects wrappedHandler using basic authentication.
func BasicAuth(wrappedHandler http.Handler, user, password string) (http.HandlerFunc, error) {
	if err := cry.IsSecure(password); err != nil {
		return nil, err
	}
	if strings.EqualFold(user, password) {
		return nil, errors.New("ong/middleware/auth: user should not equal password")
	}

	// See: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/WWW-Authenticate
	realm := `enter username and password` // Shouldn't contain 'weird' chars otherwise may break mobile browsers; https://github.com/komuw/ong/pull/457
	e := func(w http.ResponseWriter) {
		errMsg := `Basic realm=` + realm
		w.Header().Set("WWW-Authenticate", errMsg)
		w.Header().Set(ongMiddlewareErrorHeader, errMsg)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}

	f := func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if u == "" || p == "" || !ok {
			e(w)
			return
		}

		if subtle.ConstantTimeCompare([]byte(u), []byte(user)) != 1 {
			e(w)
			return
		}

		if subtle.ConstantTimeCompare([]byte(p), []byte(password)) != 1 {
			e(w)
			return
		}

		wrappedHandler.ServeHTTP(w, r)
	}

	return f, nil
}
