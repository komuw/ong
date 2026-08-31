package middleware

import (
	"context"
	"fmt"
	"net/http"

	"github.com/komuw/ong/id"
)

type cspContextKey string

const (
	cspCtxKey           = cspContextKey("cspContextKey")
	cspDefaultNonce     = ""
	cspHeader           = "Content-Security-Policy"
	cspBytesTokenLength = csrfBytesTokenLength
)

// csp is a middleware that sets Content-Security-Policy and adds its nonce to the request context.
func csp(wrappedHandler http.Handler, domain string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

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
			// - https://securityheaders.com/
			//
			// content is only permitted from:
			// - the document's origin(and subdomains)
			// - images may load from anywhere
			// - media is allowed from domain(and its subdomains)
			// - executable scripts is only allowed from self(& subdomains).
			// - DOM xss(eg setting innerHtml) is blocked by require-trusted-types.
			getCsp(domain, nonce),
		)

		wrappedHandler.ServeHTTP(w, r)
	}
}

// GetCspNonce returns the Content-Security-Policy nonce that was set for the http request in question.
func GetCspNonce(c context.Context) string {
	v := c.Value(cspCtxKey)
	if v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return cspDefaultNonce
}

func getCsp(domain, nonce string) string {
	// This csp only permitts content from:
	// - the document's origin(and subdomains)
	// - images may load from anywhere
	// - media is allowed from domain(and its subdomains)
	// - executable scripts is only allowed from self(& subdomains).
	// - DOM xss(eg setting innerHtml) is blocked by require-trusted-types.
	//
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/CSP
	return fmt.Sprintf(
		// It does not work if they are not all in same line.
		"default-src 'self' %s *.%s; img-src 'self' *; media-src 'self' %s *.%s; object-src 'none'; base-uri 'none'; require-trusted-types-for 'script'; script-src 'self' %s *.%s 'unsafe-inline' 'nonce-%s';",
		domain, domain,
		domain, domain,
		domain, domain, nonce,
	)
}
