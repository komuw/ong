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
		w.Header().Set(
			permissionsPolicyHeader,
			// flocOptOut disables floc which is otherwise ON by default
			// see: https://github.com/WICG/floc#opting-out-of-computation
			"interest-cohort=()",
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

func getSts(age time.Duration) string {
	dur := int64(age.Seconds())
	return fmt.Sprintf(`max-age=%d; includeSubDomains; preload`, dur)
}
