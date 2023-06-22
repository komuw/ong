package server

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/komuw/ong/internal/dmn"

	"golang.org/x/crypto/acme"
)

// Most of the code here is inspired(or taken from) by:
//   (a) https://github.com/eliben/code-for-blog whose license(Unlicense) can be found here:                                   https://github.com/eliben/code-for-blog/blob/464a32f686d7646ba3fc612c19dbb550ec8a05b1/LICENSE
//   (b) https://github.com/FiloSottile/mkcert   whose license(BSD 3-Clause ) can be found here:                               https://github.com/FiloSottile/mkcert/blob/v1.4.4/LICENSE
//   (c) https://github.com/golang/crypto/blob/master/acme/autocert/autocert.go whose license(BSD 3-Clause) can be found here: https://github.com/golang/crypto/blob/05595931fe9d3f8894ab063e1981d28e9873e2cb/LICENSE
//   (d) https://github.com/caddyserver/certmagic/blob/master/handshake.go whose license(Apache 2.0) can be found here:        https://github.com/caddyserver/certmagic/blob/v0.16.1/LICENSE.txt
//

// acmeHandler returns a Handler that will handle ACME [http-01] challenge requests using acmeH
// and handles normal requests using appHandler.
//
// ACME CA sends challenge requests to `/.well-known/acme-challenge/` uri.
// Note that this `http-01` challenge does not allow [wildcard] certificates.
//
// [http-01]: https://letsencrypt.org/docs/challenge-types/
// [wildcard]: https://letsencrypt.org/docs/faq/#does-let-s-encrypt-issue-wildcard-certificates
func acmeHandler(
	appHandler http.Handler,
	acmeH func(fallback http.Handler) http.Handler,
) http.HandlerFunc {
	// todo: should we move this to `ong/middleware`?
	return func(w http.ResponseWriter, r *http.Request) {
		// This code is taken from; https://github.com/golang/crypto/blob/v0.10.0/acme/autocert/autocert.go#L398-L401
		if strings.HasPrefix(r.URL.Path, "/.well-known/acme-challenge/") && acmeH != nil {
			acmeH(appHandler).ServeHTTP(w, r)
			return
		}

		appHandler.ServeHTTP(w, r)
	}
}

// getTlsConfig returns a proper tls configuration given the options passed in.
// The tls config may either procure certifiates from ACME, from disk or be nil(for non-tls traffic)
//
// h is the fallback is the http handler that will be delegated to for non ACME requests.
func getTlsConfig(o Opts) (c *tls.Config, acmeH func(fallback http.Handler) http.Handler, e error) {
	defer func() {
		// see: https://go.dev/play/p/3orL3CyP9a8
		if o.tls.acmeEmail != "" { // This is ACME
			if acmeH == nil && e == nil {
				e = errors.New("ong/server: acme could not be setup properly")
			}
		}
	}()

	if err := dmn.Validate(o.tls.domain); err != nil {
		return nil, nil, err
	}

	if o.tls.acmeEmail != "" {
		// 1. use ACME.
		//
		if o.tls.url == "" {
			return nil, nil, errors.New("ong/server: acmeURL cannot be empty if acmeEmail is also specified")
		}

		cm := dmn.CertManager(o.tls.domain, o.tls.acmeEmail, o.tls.url)
		tlsConf := &tls.Config{
			// taken from:
			// https://github.com/golang/crypto/blob/05595931fe9d3f8894ab063e1981d28e9873e2cb/acme/autocert/autocert.go#L228-L234
			NextProtos: []string{
				"h2", // enable HTTP/2
				"http/1.1",
				acme.ALPNProto, // enable tls-alpn ACME challenges
			},
			GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
				// GetCertificate returns a Certificate based on the given ClientHelloInfo.
				// it is called if `tls.Config.Certificates` is empty.
				//
				setFingerprint(info)

				c, err := cm.GetCertificate(info)
				if err != nil {
					// This will be logged by `http.Server.ErrorLog`
					err = fmt.Errorf(
						"ong/server: failed to get certificate from ACME. acmeURL=%s. domain=%s. serverName=%s. : %w",
						o.tls.url,
						o.tls.domain,
						info.ServerName,
						err,
					)
				}

				return c, err
			},
		}

		return tlsConf, cm.HTTPHandler, nil
	}
	if o.tls.certFile != "" {
		// 2. get from disk.
		//
		if len(o.tls.keyFile) < 1 {
			return nil, nil, errors.New("ong/server: keyFile cannot be empty if certFile is also specified")
		}
		c, err := tls.LoadX509KeyPair(o.tls.certFile, o.tls.keyFile)
		if err != nil {
			return nil, nil, err
		}

		tlsConf := &tls.Config{
			NextProtos: []string{
				"h2", // enable HTTP/2
				"http/1.1",
				acme.ALPNProto, // enable tls-alpn ACME challenges
			},
			GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
				// GetCertificate returns a Certificate based on the given ClientHelloInfo.
				// it is called if `tls.Config.Certificates` is empty.
				//
				setFingerprint(info)

				return &c, nil
			},
		}
		return tlsConf, nil, nil
	}

	// 3. non-tls traffic.
	return nil, nil, errors.New("ong/server: ong only serves https")
}
