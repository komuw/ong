package middleware

import (
	"fmt"
	"net/http"
)

// HttpsRedirector is a middleware that redirects http requests to https.
func HttpsRedirector(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil {
			// r.URL.Scheme = "https"
			path := fmt.Sprintf("https://%s%s", r.Host, r.URL.String())
			fmt.Println(path, r.Host, r.URL.String())
			http.Redirect(w, r, path, http.StatusPermanentRedirect)
			return
		}

		wrappedHandler(w, r)
	}
}
