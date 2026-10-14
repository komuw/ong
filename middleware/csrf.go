package middleware

import (
	"fmt"
	"net/http"
	"slices"
)

// csrf is a middleware that provides protection against Cross Site Request Forgeries.
//
// It rejects non-safe cross-origin browser requests with a http 403. It uses [http.CrossOriginProtection]
// which detects cross-origin requests using the Sec-Fetch-Site header, or by comparing the Origin header with the Host header.
// Requests from allowedOrigins are trusted. The wildcard allowedOrigin "*" does not trust any extra origin.
//
// see:
//   - https://words.filippo.io/csrf/
//   - https://github.com/komuw/ong/issues/495
func csrf(
	wrappedHandler http.Handler,
	allowedOrigins []string,
	domain string,
) http.HandlerFunc {
	// Validation of arguments should already have happened in `github.com/komuw/ong/config`
	// It makes sure that "*" can only be used on its own, and that all other allowedOrigins are exact origins.
	allowedOrigins = getOrigins(allowedOrigins, domain)

	cop := http.NewCrossOriginProtection()

	// allowedOrigins are shared with the cors middleware.
	// For cors, "*" means that any origin can read responses to requests that do not have credentials(eg cookies).
	// `config` does not allow "*" together with allowCredentials.
	// csrf protects requests that have credentials and that change state. A cross-site html form POST is sent by
	// browsers with the user's cookies, and cors does not block it.
	// So "*" is not a reason to trust every origin for csrf. If we did, an application that uses "*" for a public API would
	// lose csrf protection for all its forms(eg login). Thus, for "*" we trust no extra origin; same-origin requests
	// and requests that are not from browsers are still allowed by `CrossOriginProtection`.
	if !slices.Equal(allowedOrigins, []string{"*"}) {
		for _, origin := range allowedOrigins {
			if err := cop.AddTrustedOrigin(origin); err != nil {
				// This should not happen. `config.validateAllowedOrigins` rejects the origins that AddTrustedOrigin rejects.
				panic(fmt.Errorf("ong/middleware/csrf: %w", err))
			}
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
