package middleware

import (
	"net/http"
)

// HttpsRedirector is a middleware that redirects http requests to https.
func HttpsRedirector(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil {
			// http.RedirectHandler(url string, code int)
			http.Redirect(w, r, r.URL.Host, http.StatusPermanentRedirect)
			return
		}

		wrappedHandler(w, r)
	}
}
