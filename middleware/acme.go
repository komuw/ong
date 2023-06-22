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
	cm := dmn.CertManager(domain, acmeEmail, acmeDirectoryUrl)
	acmeH := cm.HTTPHandler

	return func(w http.ResponseWriter, r *http.Request) {
		// This code is taken from; https://github.com/golang/crypto/blob/v0.10.0/acme/autocert/autocert.go#L398-L401
		if strings.HasPrefix(r.URL.Path, "/.well-known/acme-challenge/") && acmeH != nil {
			acmeH(wrappedHandler).ServeHTTP(w, r)
			return
		}

		wrappedHandler.ServeHTTP(w, r)
	}
}
