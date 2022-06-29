package middleware

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
)

// HttpsRedirector is a middleware that redirects http requests to https.
func HttpsRedirector(httpsPort string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		isTls := strings.EqualFold(r.URL.Scheme, "https") || r.TLS != nil

		if !isTls {
			url := r.URL
			url.Scheme = "https"

			host, _, err := net.SplitHostPort(r.Host)
			if err != nil {
				host = r.Host
			}
			host = net.JoinHostPort(host, httpsPort)
			url.Host = host
			path := url.String()
			fmt.Println("\t HttpsRedirector: ", path)

			http.Redirect(w, r, path, http.StatusPermanentRedirect)
			return
		}

		// This part should never be reached.
		//
		errHttpsRedirector := errors.New(
			// this error is inspired by this one from the Go stdlib:
			// https://github.com/golang/go/blob/go1.18.3/src/net/http/server.go#L1853
			"Client sent a HTTPS request to the HttpsRedirector middleware.",
		)
		w.Header().Set(gowebMiddlewareErrorHeader, errHttpsRedirector.Error())
		http.Error(
			w,
			errHttpsRedirector.Error(),
			http.StatusBadRequest,
		)
	}
}
