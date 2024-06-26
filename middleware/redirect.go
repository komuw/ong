package middleware

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// httpsRedirector is a middleware that redirects http requests to https.
// It also protects against [DNS rebinding] attacks.
//
// domain is the domain name of your website.
// httpsPort is the tls port where http requests will be redirected to.
//
// [DNS rebinding]: https://en.wikipedia.org/wiki/DNS_rebinding
func httpsRedirector(wrappedHandler http.Handler, httpsPort uint16, domain string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host, _ := getHostPort(r.Host)

		/*
		   The protections should happen in the order listed.
		   - IP should be redirected to domain
		   - Then DNS rebinding protection has to happen b4 http->https redirect.
		   See: https://github.com/komuw/ong/issues/337

		   There's still a small problem, suppose your domain is `good.com` at IP `87.45.2.3`
		   a malicious actor could send the request.
		       curl -vkL -H 'Host: 87.45.2.3' http://bad.com
		   This middleware will redirect it to `https://good.com`
		   So in a perfect world, you would also want to make sure that you only redirect IP addresses
		   that you are in control of.
		   Maybe by doing `net.LookupIP("good.com")`. But that seems like too much work for little gain??
		   If you think about it, a malicious actor could setup `bad.com` on their own IP address and then in
		   their webserver, redirect all requests to `good.com`. As the owner of `good.com` there's nothing you can do about it.
		*/

		{ // 1. bareIP -> https redirect.
			// A Host header field must be sent in all HTTP/1.1 request messages.
			// Thus we expect `r.Host[0]` to always have a value.
			// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Host
			if _, err := netip.ParseAddr(host); err == nil {
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
				url.Host = joinHostPort(domain, fmt.Sprint(httpsPort))
				path := url.String()

				http.Redirect(w, r, path, http.StatusPermanentRedirect)
				return
			}
		}

		{ // 2. DNS rebinding attack protection.
			// todo: before calling `isDomainOrSubdomain` we need to make sure that both args are already be in canonical form.
			// see; https://github.com/golang/go/blob/master/src/net/http/client.go#L1001-L1003
			// We know that domain is kinda already canonical since [New] validates that. But host is not.
			if !isDomainOrSubdomain(host, domain) {
				err := fmt.Errorf("ong/middleware/redirect: the HOST http header has an unexpected value: %s", host)
				w.Header().Set(ongMiddlewareErrorHeader, err.Error())
				http.Error(
					w,
					err.Error(),
					http.StatusNotFound,
				)
				return
			}
		}

		{ // 3. http -> https redirect.
			isTls := strings.EqualFold(r.URL.Scheme, "https") || r.TLS != nil
			if !isTls {
				url := r.URL
				url.Scheme = "https"
				url.Host = joinHostPort(host, fmt.Sprint(httpsPort))
				path := url.String()

				http.Redirect(w, r, path, http.StatusPermanentRedirect)
				return
			}
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

// getHostPort returns host and port.
// It is based on `http.stripHostPort` from https://github.com/golang/go/blob/go1.20.5/src/net/http/server.go#L2348-L2349
func getHostPort(h string) (host, port string) {
	// If no port on host, return unchanged
	if !strings.Contains(h, ":") {
		return h, ""
	}

	hst, prt, err := net.SplitHostPort(h)
	if err != nil {
		return h, prt // on error, return unchanged
	}

	return hst, prt
}

// isDomainOrSubdomain reports whether sub is a subdomain (or exact
// match) of the parent domain.
// Both domains must already be in canonical form.
// // It is based on `http.isDomainOrSubdomain` from https://github.com/golang/go/blob/master/src/net/http/client.go#L1009-L1013
func isDomainOrSubdomain(sub, parent string) bool {
	if sub == parent {
		return true
	}

	// If sub is "foo.example.com" and parent is "example.com",
	// that means sub must end in "."+parent.
	// Do it without allocating.
	if !strings.HasSuffix(sub, parent) {
		return false
	}

	return sub[len(sub)-len(parent)-1] == '.'
}
