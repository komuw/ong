package middleware

import (
	"context"
	"net/http"

	"github.com/komuw/ong/config"
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
func csp(wrappedHandler http.Handler, domain string, policy config.CSPPolicyFunc) http.HandlerFunc {
	if policy == nil {
		policy = config.DefaultCSPPolicy
	}

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
		w.Header().Set(cspHeader, policy(domain, nonce))

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
