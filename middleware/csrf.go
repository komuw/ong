package middleware

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/komuw/ong/cookie"
	"github.com/komuw/ong/cry"
	"github.com/komuw/ong/id"
)

// Most of the code here is inspired by(or taken from):
//   (a) https://github.com/gofiber/fiber whose license(MIT) can be found here:            https://github.com/gofiber/fiber/blob/v2.34.1/LICENSE
//   (b) https://github.com/django/django   whose license(BSD 3-Clause) can be found here: https://github.com/django/django/blob/4.0.5/LICENSE

var (
	// errCsrfTokenNotFound is returned when a request using a non-safe http method
	// either does not supply a csrf token, or the supplied token is not recognized by the server.
	errCsrfTokenNotFound    = errors.New("ong/middleware: csrf token not found")
	errCsrfTokenExpired     = errors.New("ong/middleware: csrf token is expired")
	errCsrfTokenWrongFormat = errors.New("ong/middleware: csrf token wrong format")
	once                    sync.Once //nolint:gochecknoglobals
	enc                     cry.Enc   //nolint:gochecknoglobals
)

type csrfContextKey string

const (
	// CsrfTokenFormName is the name of the html form name attribute for csrf token.
	CsrfTokenFormName = "csrftoken" // named after what django uses.
	// CsrfHeader is the name of the http header that Ong uses to store csrf token.
	CsrfHeader               = "X-Csrf-Token" // named after what fiber uses.
	csrfCtxKey               = csrfContextKey("csrfContextKey")
	csrfDefaultToken         = ""
	csrfCookieName           = CsrfTokenFormName
	clientCookieHeader       = "Cookie"
	varyHeader               = "Vary"
	authorizationHeader      = "Authorization"
	proxyAuthorizationHeader = "Proxy-Authorization"
	ctHeader                 = "Content-Type"
	formUrlEncoded           = "application/x-www-form-urlencoded"
	multiformData            = "multipart/form-data"

	// gorilla/csrf; 12hrs
	// django: 1yr??
	// gofiber/fiber; 1hr
	tokenMaxAge = 12 * time.Hour

	// django appears to use 32 random characters for its csrf token.
	// so does gorilla/csrf; https://github.com/gorilla/csrf/blob/v1.7.1/csrf.go#L13-L14
	csrfBytesTokenLength = 32

	// This value should not be changed without thinking about it.
	// This has to be a character that `id.Random()` cannot generate.
	// The cookie spec allows a sequence of characters excluding semi-colon, comma and white space.
	// So `sep` should not be any of those.
	sep = ":"
)

// csrf is a middleware that provides protection against Cross Site Request Forgeries.
//
// If a csrf token is not provided(or is not valid), when it ought to have been; this middleware will issue a http GET redirect to the same url.
func csrf(wrappedHandler http.Handler, secretKey, domain string) http.HandlerFunc {
	once.Do(func() {
		enc = cry.New(secretKey)
	})
	msgToEncrypt := id.Random(16)

	return func(w http.ResponseWriter, r *http.Request) {
		// - https://docs.djangoproject.com/en/4.0/ref/csrf/
		// - https://github.com/django/django/blob/4.0.5/django/middleware/csrf.py
		// - https://github.com/gofiber/fiber/blob/v2.34.1/middleware/csrf/csrf.go

		// 1. check http method.
		//     - if it is a 'safe' method like GET, try and get `actualToken` from request.
		//     - if it is not a 'safe' method, try and get `actualToken` from header/cookies/httpForm
		//        - take the found token and try to get it from memory store.
		//            - if not found in memory store, delete the cookie & return an error.

		ctx := r.Context()

		switch r.Method {
		// safe methods under rfc7231: https://datatracker.ietf.org/doc/html/rfc7231#section-4.2.1
		case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
			break
		default:
			// For POST requests, we insist on a CSRF cookie, and in this way we can avoid all CSRF attacks, including login CSRF.
			actualToken := getToken(r)

			ct, _, err := mime.ParseMediaType(r.Header.Get(ctHeader))
			if err == nil &&
				ct != formUrlEncoded &&
				ct != multiformData &&
				r.Header.Get(clientCookieHeader) == "" &&
				r.Header.Get(authorizationHeader) == "" &&
				r.Header.Get(proxyAuthorizationHeader) == "" {
				// For POST requests that;
				// - are not form data.
				// - have no cookies.
				// - are not using http authentication.
				// then it is okay to not validate csrf for them.
				// This is especially useful for REST API endpoints.
				// see: https://github.com/komuw/ong/issues/76
				break
			}

			tokVal, errN := enc.DecryptDecode(actualToken)
			if errN != nil {
				// We should redirect the request since it means that the server is not aware of such a token.
				// It shoulbe be a temporary redirect to the same page but this time send a http GET request.
				//
				// To test using curl, use;
				//   curl -kL \
				//   -H "Content-Type: application/x-www-form-urlencoded" \
				//   -d "firstName=john&csrftoken=bogusToken" https://localhost:65081/login/
				// Do NOT use `-X POST`, see: https://stackoverflow.com/a/41890653/2768067
				//
				cookie.Delete(w, csrfCookieName, domain)
				w.Header().Set(ongMiddlewareErrorHeader, errCsrfTokenNotFound.Error())
				http.Redirect(
					w,
					r,
					r.URL.String(),
					// http 303(StatusSeeOther) is guaranteed by the spec to always use http GET.
					// https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/303
					http.StatusSeeOther,
				)
				return
			}

			res := strings.Split(tokVal, sep)
			if len(res) != 2 {
				cookie.Delete(w, csrfCookieName, domain)
				w.Header().Set(ongMiddlewareErrorHeader, errCsrfTokenWrongFormat.Error())
				http.Redirect(w, r, r.URL.String(), http.StatusSeeOther)
				return
			}

			expires, errP := strconv.ParseInt(res[1], 10, 64)
			if errP != nil {
				cookie.Delete(w, csrfCookieName, domain)
				w.Header().Set(ongMiddlewareErrorHeader, errP.Error())
				http.Redirect(w, r, r.URL.String(), http.StatusSeeOther)
				return
			}

			diff := expires - time.Now().UTC().Unix()
			if diff <= 0 {
				cookie.Delete(w, csrfCookieName, domain)
				w.Header().Set(ongMiddlewareErrorHeader, errCsrfTokenExpired.Error())
				http.Redirect(w, r, r.URL.String(), http.StatusSeeOther)
				return
			}
		}

		// 2. generate a new token.
		/*
			We need to try and protect against BreachAttack[1]. See[2] for a refresher on how it works.
			The mitigations against the attack in order of effectiveness are:
			(a) Disabling HTTP compression
			(b) Separating secrets from user input
			(c) Randomizing secrets per request
			(d) Masking secrets (effectively randomizing by XORing with a random secret per request)
			(e) Protecting vulnerable pages with CSRF
			(f) Length hiding (by adding random number of bytes to the responses)
			(g) Rate-limiting the requests
			Most csrf implementation use (d). Here, we'll use (c)
			The [encrypt] func uses a random nonce everytime it is called.

			1. http://breachattack.com/
			2. https://security.stackexchange.com/a/172646
		*/
		expires := strconv.FormatInt(
			time.Now().UTC().Add(tokenMaxAge).Unix(),
			10,
		)
		tokenToIssue := enc.EncryptEncode(
			// see: https://github.com/golang/net/blob/v0.8.0/xsrftoken/xsrf.go#L33-L46
			fmt.Sprintf("%s%s%s", msgToEncrypt, sep, expires),
		)

		// 3. create cookie
		cookie.Set(
			w,
			csrfCookieName,
			tokenToIssue,
			domain,
			tokenMaxAge,
			true, // accessible to javascript
		)

		// 4. set cookie header
		w.Header().Set(
			CsrfHeader,
			tokenToIssue,
		)

		// 5. update Vary header.
		w.Header().Add(varyHeader, clientCookieHeader)

		// 6. store tokenToIssue in context
		r = r.WithContext(context.WithValue(ctx, csrfCtxKey, tokenToIssue))

		wrappedHandler.ServeHTTP(w, r)
	}
}

// GetCsrfToken returns the csrf token that was set for the http request in question.
func GetCsrfToken(c context.Context) string {
	v := c.Value(csrfCtxKey)
	if v != nil {
		s, ok := v.(string)
		if ok {
			return s
		}
	}
	return csrfDefaultToken
}

// getToken tries to fetch a csrf token from the incoming request r.
// It tries to fetch from cookies, http-forms, headers in that order.
func getToken(r *http.Request) (actualToken string) {
	fromCookie := func() string {
		c, err := r.Cookie(csrfCookieName)
		if err != nil {
			return ""
		}
		return c.Value
	}

	fromForm := func() string {
		return r.FormValue(CsrfTokenFormName) // calls ParseMultipartForm and ParseForm if necessary
	}

	fromHeader := func() string {
		return r.Header.Get(CsrfHeader)
	}

	tok := fromCookie()
	if tok == "" {
		tok = fromForm()
	}
	if tok == "" {
		tok = fromHeader()
	}

	if len(tok) < csrfBytesTokenLength {
		// Request has presented a token that we probably didn't generate coz this library issues
		// tokens with len > csrfBytesTokenLength
		tok = ""
	}

	return tok
}
