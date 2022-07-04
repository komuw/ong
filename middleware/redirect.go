package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"unicode"
)

// HttpsRedirector is a middleware that redirects http requests to https.
func HttpsRedirector(wrappedHandler http.Handler, httpsPort uint16, domain string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		isTls := strings.EqualFold(r.URL.Scheme, "https") || r.TLS != nil
		if !isTls {
			url := r.URL
			url.Scheme = "https"
			url.Host = joinHostPort(domain, fmt.Sprint(httpsPort))
			path := url.String()

			http.Redirect(w, r, path, http.StatusPermanentRedirect)
			return
		}

		isHostBareIP := unicode.IsDigit(rune(r.Host[0]))
		if isHostBareIP {
			/*
				the request has tried to access us via an IP address, redirect them to our domain.

				curl -vkIL 172.217.170.174 #google
				HEAD / HTTP/1.1
				Host: 172.217.170.174

				HTTP/1.1 301 Moved Permanently
				Location: http://www.google.com/
			*/
			url := r.URL
			url.Scheme = "https"
			_, port, err := net.SplitHostPort(r.Host)
			if err != nil {
				port = fmt.Sprint(httpsPort)
			}
			url.Host = joinHostPort(domain, port)
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
