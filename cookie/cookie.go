// Package cookie provides utilities for using HTTP cookies.
package cookie

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/komuw/ong/cry"
	"github.com/komuw/ong/internal/clientip"
	"github.com/komuw/ong/internal/finger"
	"github.com/komuw/ong/internal/octx"
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
// If mAge == 0, a session cookie is created. If mAge < 0, it means delete the cookie now.
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
	// Since expires is relative to the browser & we are calculating it on the server-side;
	// there's a possibility of it not doing what u expect.
	// However, browsers usually ignore this in place of maxAge.
	expires := time.Now().UTC().Add(mAge)
	maxAge := int(mAge.Seconds())

	if mAge == 0 {
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

// Get returns a copy of the named cookie.
func Get(r *http.Request, name string) (*http.Cookie, error) {
	c, err := r.Cookie(name)
	if err != nil {
		return nil, err
	}

	c2 := &http.Cookie{
		Name:     c.Name,
		Value:    c.Value,
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

// This value should not be changed without thinking about it.
// This has to be a character that `id.Random()` cannot generate.
// The cookie spec allows a sequence of characters excluding semi-colon, comma and white space.
// So `sep` should not be any of those.
const sep = ":"

var (
	enc  cry.Enc   //nolint:gochecknoglobals
	once sync.Once //nolint:gochecknoglobals
)

// SetEncrypted creates a cookie on the HTTP response.
// The cookie value(but not the name) is encrypted and authenticated using [cry.Enc].
//
// Note: While encrypted cookies can guarantee that the data has not been tampered with,
// that it is all there and correct, and that the clients cannot read its raw value; they cannot guarantee freshness.
// This means that (similar to plain-text cookies), they are still susceptible to [replay attacks]
//
// Also see [Set]
//
// [replay attacks]: https://en.wikipedia.org/wiki/Replay_attack
func SetEncrypted(
	r *http.Request,
	w http.ResponseWriter,
	name string,
	value string,
	domain string,
	mAge time.Duration,
	secretKey string,
) {
	once.Do(func() {
		enc = cry.New(secretKey)
	})

	antiReplay := getAntiReplay(r)
	expires := strconv.FormatInt(
		time.Now().UTC().Add(mAge).Unix(),
		10,
	)
	combined := antiReplay + expires + value

	encryptedEncodedVal := fmt.Sprintf(
		"%d%s%d%s%s",
		len(antiReplay),
		sep,
		len(expires),
		sep,
		enc.EncryptEncode(combined),
	)

	Set(
		w,
		name,
		encryptedEncodedVal,
		domain,
		mAge,
		false,
	)
}

// GetEncrypted authenticates, un-encrypts and returns a copy of the named cookie with the value decrypted.
func GetEncrypted(
	r *http.Request,
	name string,
	secretKey string,
) (*http.Cookie, error) {
	once.Do(func() {
		enc = cry.New(secretKey)
	})

	c, err := Get(r, name)
	if err != nil {
		return nil, err
	}

	subs := strings.Split(c.Value, sep)
	if len(subs) != 3 {
		return nil, errors.New("ong/cookie: invalid cookie")
	}

	lenOfAntiReplay, err := strconv.Atoi(subs[0])
	if err != nil {
		return nil, err
	}

	lenOfExpires, err := strconv.Atoi(subs[1])
	if err != nil {
		return nil, err
	}

	decryptedVal, err := enc.DecryptDecode(subs[2])
	if err != nil {
		return nil, err
	}

	antiReplay, expiresStr, val := decryptedVal[:lenOfAntiReplay],
		decryptedVal[lenOfAntiReplay:lenOfAntiReplay+lenOfExpires],
		decryptedVal[lenOfAntiReplay+lenOfExpires:]

	{
		// Try and prevent replay attacks & session hijacking(https://twitter.com/4A4133/status/1615103474739429377)
		// This does not completely stop them, but it is better than nothing.
		incomingAntiReplay := getAntiReplay(r)
		if antiReplay != incomingAntiReplay {
			return nil, errors.New("ong/cookie: mismatched anti replay value")
		}

		expires, errP := strconv.ParseInt(expiresStr, 10, 64)
		if errP != nil {
			return nil, errP
		}

		// You cannot trust anything about the incoming cookie except its value.
		// This is because, it is the only thing that was encrypted/authenticated.
		// So we cannot use `c.MaxAge` here, since a client could have modified that.
		diff := expires - time.Now().UTC().Unix()
		if diff <= 0 {
			return nil, errors.New("ong/cookie: cookie is expired")
		}
	}

	c.Value = val
	return c, nil
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

// getAntiReplay fetched any antiReplay data from [http.Request].
func getAntiReplay(r *http.Request) string {
	ctx := r.Context()
	if vCtx := ctx.Value(octx.AntiReplayCtxKey); vCtx != nil {
		if s, ok := vCtx.(string); ok {
			return s
		}
	}
	return ""
}

// SetAntiReplay uses antiReplay to try and mitigate against [replay attacks].
// This mitigation not foolproof.
//
// [replay attacks]: https://en.wikipedia.org/wiki/Replay_attack
func SetAntiReplay(r *http.Request, antiReplay string) *http.Request {
	ctx := r.Context()
	ctx = context.WithValue(ctx, octx.AntiReplayCtxKey, antiReplay)
	r = r.WithContext(ctx)

	return r
}

// UseClientAntiReplay uses the client IP address and client TLS fingerprint to try and mitigate against [replay attacks].
//
// [replay attacks]: https://en.wikipedia.org/wiki/Replay_attack
func UseClientAntiReplay(r *http.Request) *http.Request {
	ip := clientip.Get(
		// Note:
		//   - client IP can be spoofed easily and this could lead to issues with their cookies.
		//   - also it means that if someone moves from wifi internet to phone internet, their IP changes and cookie/session will be invalid.
		r,
	)
	fingerprint := finger.Get(
		// might also be spoofed??
		r,
	)

	return SetAntiReplay(r, fmt.Sprintf("%s:%s", ip, fingerprint))
}
