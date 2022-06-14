// Package cookie provides utilities for using HTTP cookies.
package cookie

import (
	"net/http"
	"time"
)

const (
	serverCookieHeader  = "Set-Cookie"
	clientCookieHeader  = "Cookie"
	maxCookiesPerDomain = 50
	maxCookieSize       = 4096 // bytes
	// see: https://datatracker.ietf.org/doc/html/rfc6265#section-6.1
)

// Set creates a cookie on the HTTP response.
func Set(
	w http.ResponseWriter,
	name string,
	value string,
	domain string,
	maxAge time.Duration,
	httpOnly bool,
) {
	c := &http.Cookie{
		Name:  name,
		Value: value,
		// If Domain is omitted(empty string), it defaults to the current host, excluding including subdomains.
		// If a domain is specified, then subdomains are always included.
		Domain: domain,
		// Expires is relative to the client the cookie is being set on, not the server.
		Expires: time.Now().Add(maxAge),
		// Every browser that supports MaxAge will ignore Expires regardless of it's value
		// https://datatracker.ietf.org/doc/html/rfc2616#section-13.2.4
		MaxAge: int(maxAge.Seconds()),
		Path:   "/",

		// Security
		HttpOnly: httpOnly, // If true, makes cookie inaccessible to JS. Should be false for csrf cookies.
		Secure:   true,     // https only.
		SameSite: http.SameSiteStrictMode,
	}

	// Session cookies are those that do not specify the Expires or Max-Age attribute.
	// Session cookies are removed when the client shuts down(session ends).
	// The browser defines when the "current session" ends,
	// some browsers use session restoring when restarting.
	// This can cause session cookies to last indefinitely.
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Set-Cookie#session_cookie
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Cookies#define_the_lifetime_of_a_cookie

	http.SetCookie(w, c)
}

// Delete removes the named cookie.
func Delete(w http.ResponseWriter, name, domain string) {
	h := w.Header().Values(serverCookieHeader)
	if len(h) <= 0 {
		return
	}

	if len(h) >= maxCookiesPerDomain || len(h[0]) > maxCookieSize {
		// cookies exceed limits set out in RFC6265
		w.Header().Del(serverCookieHeader)
		return
	}

	c := &http.Cookie{
		Name:    name,
		Value:   "",
		Domain:  domain,
		Path:    "/",
		MaxAge:  -1,
		Expires: time.Unix(0, 0),
	}
	http.SetCookie(w, c)
}
