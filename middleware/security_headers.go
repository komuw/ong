package middleware

import (
	"fmt"
	"net/http"
	"time"
)

const (
	// allow or block the use of browser features(eg accelerometer, camera, autoplay etc).
	permissionsPolicyHeader = "Permissions-Policy"
	xContentOptionsHeader   = "X-Content-Type-Options"
	// protect website from being embedded by any other websites.
	xFrameHeader = "X-Frame-Options"
	// protect from attacker embedding resources from another origin.
	corpHeader = "Cross-Origin-Resource-Policy"
	// protect from an attacker's website been able to open another ua site in a popup window to learn information about it.
	coopHeader     = "Cross-Origin-Opener-Policy"
	referrerHeader = "Referrer-Policy"
	stsHeader      = "Strict-Transport-Security"
)

// securityHeaders is a middleware that sets HTTP security headers other than Content-Security-Policy.
func securityHeaders(wrappedHandler http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// - https://developer.mozilla.org/en-US/docs/Web/HTTP/CSP
		// - https://web.dev/security-headers/
		// - https://stackoverflow.com/a/66955464/2768067
		// - https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Security-Policy/script-src
		// - https://web.dev/security-headers/#tt
		// - https://securityheaders.com/
		//

		h := w.Header()

		// flocOptOut disables floc which is otherwise ON by default
		// see: https://github.com/WICG/floc#opting-out-of-computation
		setDefaultHeader(h, permissionsPolicyHeader, "interest-cohort=()")
		setDefaultHeader(h, xContentOptionsHeader, "nosniff")
		setDefaultHeader(h, xFrameHeader, "DENY")
		setDefaultHeader(h, corpHeader, "same-site")
		setDefaultHeader(h, coopHeader, "same-origin")
		setDefaultHeader(h, referrerHeader, "strict-origin-when-cross-origin")

		if r.TLS != nil {
			// A max-age(in seconds) of 2yrs is recommended
			setDefaultHeader(h, stsHeader, getSts(60*24*time.Hour))
		}

		wrappedHandler.ServeHTTP(w, r)
	}
}

func setDefaultHeader(h http.Header, name, value string) {
	if h.Get(name) == "" {
		h.Set(name, value)
	}
}

func getSts(age time.Duration) string {
	dur := int64(age.Seconds())
	return fmt.Sprintf(`max-age=%d; includeSubDomains; preload`, dur)
}
