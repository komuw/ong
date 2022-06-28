package middleware

import (
	"net/http"
)

// HttpsRedirector is a middleware that redirects http requests to https.
func HttpsRedirector(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil {
			http.Redirect(w, r, r.URL.String(), http.StatusPermanentRedirect)
			return
		}

		wrappedHandler(w, r)
	}
}
