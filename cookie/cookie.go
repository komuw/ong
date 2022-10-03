// Package cookie provides utilities for using HTTP cookies.
package cookie

import (
	"net/http"
	"sync"
	"time"

	"github.com/komuw/ong/cry"
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
// If domain is an empty string, the cookie is set for the current host(excluding subdomains) else it is set for the given domain and its subdomains.
// If mAge <= 0, a session cookie is created. If jsAccess is false, the cookie will be in-accesible to Javascript.
// In most cases you should set it to false(exceptions are rare, like when setting a csrf cookie)
func Set(
	w http.ResponseWriter,
	name string,
	value string,
	domain string,
	mAge time.Duration,
	jsAccess bool,
) {
	// Since expires is relative to the browser & we are calculating it on the server-side;
	// there's a possibility of it not doing what u expect.
	// However, browsers usually ignore this in place of maxAge.
	expires := time.Now().UTC().Add(mAge)
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

var (
	enc  cry.Enc   //nolint:gochecknoglobals
	once sync.Once //nolint:gochecknoglobals
)

// SetEncrypted creates a cookie on the HTTP response.
// The cookie value(but not the name) is encrypted and authenticated using [cry.Enc].
//
// Note: While encrypted cookies can guarantee that the data has not been tampered with,
// that it is all there and correct, and that the clients cannot read its raw value.
// They cannot guarantee freshness i.e. that you are being sent back the last thing you sent to the client.
// This means that (similar to plain-text cookies), it is still susceptible to [replay attacks]
//
// Also see [Set]
//
// [replay attacks]: https://en.wikipedia.org/wiki/Replay_attack
func SetEncrypted(
	w http.ResponseWriter,
	name string,
	value string,
	domain string,
	mAge time.Duration,
	key string,
) {
	once.Do(func() {
		enc = cry.New(key)
	})

	encryptedEncodedVal := enc.EncryptEncode(value)
	Set(
		w,
		name,
		encryptedEncodedVal,
		domain,
		mAge,
		false,
	)
}

// GetEncrypted authenticates, un-encrypts and returns a copy of the named cookie.
func GetEncrypted(
	r *http.Request,
	name string,
	key string,
) (*http.Cookie, error) {
	once.Do(func() {
		enc = cry.New(key)
	})

	c, err := r.Cookie(name)
	if err != nil {
		return nil, err
	}

	val, err := enc.DecryptDecode(c.Value)
	if err != nil {
		return nil, err
	}

	c2 := &http.Cookie{
		Name:     c.Name,
		Value:    val,
		Path:     c.Path,
		Domain:   c.Domain,
		Expires:  c.Expires,
		MaxAge:   c.MaxAge,
		Secure:   c.Secure,
		HttpOnly: c.HttpOnly,
		SameSite: c.SameSite,
		Raw:      c.Raw,
		// do not add c.Unparsed since it is a slice of strings and caller of GetEncrypted may manipulate it.
	}

	return c2, nil
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
