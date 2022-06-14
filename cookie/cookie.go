// Package cookie provides utilities for using HTTP cookies.
package cookie

import (
	"net/http"
	"time"
)

const (
	serverCookieHeader  = "Set-Cookie"
	clientCookieHeader  = "Cookie"
	_                   = clientCookieHeader // silence unused var linter
	maxCookiesPerDomain = 50
	maxCookieSize       = 4096 // bytes
	// see: https://datatracker.ietf.org/doc/html/rfc6265#section-6.1
)

// Set creates a cookie on the HTTP response.
//
// If domain is an empty string, the cookie is set for the current host(excluding subdomains)
// else it is set for the given domain and its subdomains.
// If mAge <= 0, a session cookie is created.
// If jsAccess is false, the cookie will be in-accesible to Javascript.
// In most cases you should set it to false(exceptions are rare, like when setting a csrf cookie)
func Set(
	w http.ResponseWriter,
	name string,
	value string,
	domain string,
	mAge time.Duration,
	jsAccess bool,
) {
	expires := time.Now().Add(mAge)
	maxAge := int(mAge.Seconds())

	if mAge <= 0 {
		// this is a session cookie
		expires = time.Time{}
		maxAge = 0
	}

	httpOnly := true
	if jsAccess {
		httpOnly = false
	}

	c := &http.Cookie{
		Name:  name,
		Value: value,
		// If Domain is omitted(empty string), it defaults to the current host, excluding including subdomains.
		// If a domain is specified, then subdomains are always included.
		Domain: domain,
		// Expires is relative to the client the cookie is being set on, not the server.
		// Session cookies are those that do not specify the Expires or Max-Age attribute.
		Expires: expires,
		// Every browser that supports MaxAge will ignore Expires regardless of it's value
		// https://datatracker.ietf.org/doc/html/rfc2616#section-13.2.4
		MaxAge: maxAge,
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
