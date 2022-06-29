package middleware

import (
	"net"
	"net/http"
	"strings"
)

// HttpsRedirector is a middleware that redirects http requests to https.
func HttpsRedirector(wrappedHandler http.Handler, httpsPort string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		isTls := strings.EqualFold(r.URL.Scheme, "https") || r.TLS != nil
		if !isTls {
			url := r.URL
			url.Scheme = "https"

			host, _, err := net.SplitHostPort(r.Host)
			if err != nil {
				host = r.Host
			}
			host = joinHostPort(host, httpsPort)
			url.Host = host
			path := url.String()

			http.Redirect(w, r, path, http.StatusPermanentRedirect)
			return
		}

		wrappedHandler.ServeHTTP(w, r)
	}
}

// joinHostPort is like `net.JoinHostPort` except suited for this package.
//
// joinHostPort combines host and port into a network address of the
// form "host:port". If host contains a colon, as found in literal
// IPv6 addresses, then joinHostPort returns "[host]:port".
//
// See func Dial for a description of the host and port parameters.
func joinHostPort(host, port string) string {
	// We assume that host is a literal IPv6 address if host has
	// colons.

	sep := ":"
	if port == "443" || port == "80" || port == "" {
		port = ""
		sep = ""
	}

	if indexByteString(host, ':') >= 0 {
		return "[" + host + "]" + sep + port
	}
	return host + sep + port
}

// indexByteString is like `bytealg.IndexByteString` from golang internal packages.
func indexByteString(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}
