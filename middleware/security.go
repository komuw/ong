package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/komuw/ong/id"
)

type cspContextKey string

const (
	cspCtxKey       = cspContextKey("cspContextKey")
	cspDefaultNonce = ""

	// allow or block the use of browser features(eg accelerometer, camera, autoplay etc).
	permissionsPolicyHeader = "Permissions-Policy"
	// CSP is an added layer of security that helps to mitigate certain types of attacks, including Cross-Site Scripting & data injection attacks.
	cspHeader             = "Content-Security-Policy"
	xContentOptionsHeader = "X-Content-Type-Options"
	// protect website from being embedded by any other websites.
	xFrameHeader = "X-Frame-Options"
	// protect from attacker embedding resources from another origin.
	corpHeader = "Cross-Origin-Resource-Policy"
	// protect from an attacker's website been able to open another ua site in a popup window to learn information about it.
	coopHeader     = "Cross-Origin-Opener-Policy"
	referrerHeader = "Referrer-Policy"
	stsHeader      = "Strict-Transport-Security"

	cspBytesTokenLength = csrfBytesTokenLength
)

// securityHeaders is a middleware that adds some important HTTP security headers and assigns them sensible default values.
//
// Some of the headers set are Permissions-Policy, Content-securityHeaders-Policy, X-Content-Type-Options, X-Frame-Options, Cross-Origin-Resource-Policy, Cross-Origin-Opener-Policy, Referrer-Policy & Strict-Transport-securityHeaders
func securityHeaders(wrappedHandler http.Handler, domain string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		w.Header().Set(
			permissionsPolicyHeader,
			// flocOptOut disables floc which is otherwise ON by default
			// see: https://github.com/WICG/floc#opting-out-of-computation
			"interest-cohort=()",
		)

		// The nonce should be generated per request & propagated to the html of the page.
		// The nonce can be fetched in middlewares using the GetCspNonce func
		//
		// eg;
		// <script nonce="2726c7f26c">
		//   var inline = 1;
		// </script>
		nonce := id.Random(cspBytesTokenLength)
		r = r.WithContext(context.WithValue(ctx, cspCtxKey, nonce))
		w.Header().Set(
			cspHeader,
			// - https://developer.mozilla.org/en-US/docs/Web/HTTP/CSP
			// - https://web.dev/security-headers/
			// - https://stackoverflow.com/a/66955464/2768067
			// - https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Security-Policy/script-src
			// - https://web.dev/security-headers/#tt
			//
			// content is only permitted from:
			// - the document's origin(and subdomains)
			// - images may load from anywhere
			// - media is allowed from domain(and its subdomains)
			// - executable scripts is only allowed from self(& subdomains).
			// - DOM xss(eg setting innerHtml) is blocked by require-trusted-types.
			getCsp(domain, nonce),
		)

		w.Header().Set(
			xContentOptionsHeader,
			"nosniff",
		)

		w.Header().Set(
			xFrameHeader,
			"DENY",
		)

		w.Header().Set(
			corpHeader,
			"same-site",
		)

		w.Header().Set(
			coopHeader,
			"same-origin",
		)

		w.Header().Set(
			referrerHeader,
			// - https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Referrer-Policy
			"strict-origin-when-cross-origin",
		)

		if r.TLS != nil {
			w.Header().Set(
				stsHeader,
				// - https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Strict-Transport-Security
				// A max-age(in seconds) of 2yrs is recommended
				getSts(60*24*time.Hour), // 60 days
			)
		}

		wrappedHandler.ServeHTTP(w, r)
	}
}

// GetCspNonce returns the Content-Security-Policy nonce that was set for the http request in question.
func GetCspNonce(c context.Context) string {
	v := c.Value(cspCtxKey)
	if v != nil {
		s, ok := v.(string)
		if ok {
			return s
		}
	}
	return cspDefaultNonce
}

func getCsp(domain, nonce string) string {
	// content is only permitted from:
	// - the document's origin(and subdomains)
	// - images may load from anywhere
	// - media is allowed from domain(and its subdomains)
	// - executable scripts is only allowed from self(& subdomains).
	// - DOM xss(eg setting innerHtml) is blocked by require-trusted-types.
	return fmt.Sprintf(`
default-src 'self' %s *.%s;
img-src *;
media-src %s *.%s;
object-src 'none';
base-uri 'none';
require-trusted-types-for 'script';
script-src 'self' %s *.%s 'unsafe-inline' 'nonce-%s';`,
		domain, domain,
		domain, domain,
		domain, domain, nonce,
	)
}

func getSts(age time.Duration) string {
	dur := int64(age.Seconds())
	return fmt.Sprintf(`max-age=%d; includeSubDomains; preload`, dur)
}
