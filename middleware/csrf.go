package middleware

import (
	"fmt"
	"net/http"
)

// csrf is a middleware that provides protection against Cross Site Request Forgeries.
//
// It rejects non-safe cross-origin browser requests with a http 403. It uses [http.CrossOriginProtection]
// which detects cross-origin requests using the Sec-Fetch-Site header, or by comparing the Origin header with the Host header.
// Requests from trustedCsrfOrigins are trusted.
//
// see:
//   - https://words.filippo.io/csrf/
//   - https://github.com/komuw/ong/issues/495
func csrf(
	wrappedHandler http.Handler,
	trustedCsrfOrigins []string,
) http.HandlerFunc {
	// Validation of arguments should already have happened in `github.com/komuw/ong/config`.
	cop := http.NewCrossOriginProtection()
	for _, origin := range trustedCsrfOrigins {
		if err := cop.AddTrustedOrigin(origin); err != nil {
			// This should not happen. `config.validateTrustedCsrfOrigins` rejects any origin that AddTrustedOrigin rejects.
			panic(fmt.Errorf("ong/middleware/csrf: %w", err))
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if err := cop.Check(r); err != nil {
			w.Header().Set(ongMiddlewareErrorHeader, err.Error())
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}

		wrappedHandler.ServeHTTP(w, r)
	}
}
