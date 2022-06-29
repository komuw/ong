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

func RedirectToHTTPSRouter() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("\n\t RedirectToHTTPSRouter called..\n.")
		isTls := strings.EqualFold(r.URL.Scheme, "https") || r.TLS != nil

		if !isTls {
			// kama.Dirp(r.URL)
			// r.URL.Scheme = "https"

			fmt.Println(" r.Host: ", r.Host)
			url := r.URL
			url.Scheme = "https"

			host, _, err := net.SplitHostPort(r.Host)
			if err != nil {
				host = r.Host
			}
			httpsPort := "8081"
			host = net.JoinHostPort(host, httpsPort)
			url.Host = host

			path := url.String() // fmt.Sprintf("https://%s%s", r.Host, r.URL.String())
			fmt.Println(path)    // r.Host, r.URL.String())
			http.Redirect(w, r, path, http.StatusPermanentRedirect)
			return
		}

		// next.ServeHTTP(w, r)
	})
}
