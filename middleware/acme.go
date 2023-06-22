package middleware

import (
	"net/http"
	"strings"

	"github.com/komuw/ong/internal/dmn"
)

// Most of the code here is inspired(or taken from) by:
//   (a) https://github.com/golang/crypto/blob/master/acme/autocert/autocert.go whose license(BSD 3-Clause) can be found here: https://github.com/golang/crypto/blob/05595931fe9d3f8894ab063e1981d28e9873e2cb/LICENSE
//

// TODO: add docs.
func acme(wrappedHandler http.Handler, domain, acmeEmail, acmeDirectoryUrl string) http.HandlerFunc {
	// `dmn.CertManager` should be called with valid domain.
	// `middleware.New` validates the domain, so that by the time we get here, domain is valid.
	cm := dmn.CertManager(domain, acmeEmail, acmeDirectoryUrl)
	acmeHandler := cm.HTTPHandler
	acmeEnabled := acmeHandler != nil

	return func(w http.ResponseWriter, r *http.Request) {
		// This code is taken from; https://github.com/golang/crypto/blob/v0.10.0/acme/autocert/autocert.go#L398-L401
		if acmeEnabled && strings.HasPrefix(r.URL.Path, "/.well-known/acme-challenge/") {
			acmeHandler(wrappedHandler).ServeHTTP(w, r)
			return
		}

		wrappedHandler.ServeHTTP(w, r)
	}
}
