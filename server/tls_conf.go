package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/komuw/ong/internal/dmn"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/net/idna"
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
		if o.tls.email != "" { // This is ACME
			if acmeH == nil && e == nil {
				e = errors.New("ong/server: acme could not be setup properly")
			}
		}
	}()

	if err := dmn.Validate(o.tls.domain); err != nil {
		return nil, nil, err
	}

	if o.tls.email != "" {
		// 1. use ACME.
		//
		if o.tls.url == "" {
			return nil, nil, errors.New("ong/server: acmeURL cannot be empty if email is also specified")
		}

		m, err := dmn.CertManager(o.tls.domain, o.tls.email, o.tls.url)
		if err != nil {
			return nil, nil, err
		}

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

				c, err := m.GetCertificate(info)
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

		return tlsConf, m.HTTPHandler, nil
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

func validateDomain(domain string) error {
	if len(domain) < 1 {
		return errors.New("ong/server: domain cannot be empty")
	}
	if strings.Count(domain, "*") > 1 {
		return errors.New("ong/server: domain can only contain one wildcard character")
	}
	if strings.Contains(domain, "*") && !strings.HasPrefix(domain, "*") {
		return errors.New("ong/server: domain wildcard character should be a prefix")
	}
	if strings.Contains(domain, "*") && domain[1] != '.' {
		return errors.New("ong/server: domain wildcard character should be followed by a `.` character")
	}

	if !strings.Contains(domain, "*") {
		// not wildcard
		if _, err := idna.Lookup.ToASCII(domain); err != nil {
			return err
		}
	}

	return nil
}

// customHostWhitelist is modeled after `autocert.HostWhitelist` except that it allows wildcards.
// However, the certificate issued will NOT be wildcard certs; since letsencrypt only issues wildcard certs via DNS-01 challenge
// Instead, we'll get a certifiate per subdomain.
// see; https://letsencrypt.org/docs/faq/#does-let-s-encrypt-issue-wildcard-certificates
//
// HostWhitelist returns a policy where only the specified domain names are allowed.
//
// Note that all domain will be converted to Punycode via idna.Lookup.ToASCII so that
// Manager.GetCertificate can handle the Unicode IDN and mixedcase domain correctly.
// Invalid domain will be silently ignored.
func customHostWhitelist(domain string) autocert.HostPolicy {
	// wildcard validation has already happened in `validateDomain`
	exactMatch := ""
	wildcard := ""
	if !strings.Contains(domain, "*") {
		// not wildcard
		if h, err := idna.Lookup.ToASCII(domain); err == nil {
			exactMatch = h
		}
	} else {
		// wildcard
		wildcard = domain
		wildcard = strings.ToLower(strings.TrimSpace(wildcard))
		{
			// if wildcard is `*.example.com` we should also match `example.com`
			exactMatch = cleanDomain(domain)
			if h, err := idna.Lookup.ToASCII(exactMatch); err == nil {
				exactMatch = h
			}
		}
	}

	return func(_ context.Context, host string) error {
		host = strings.ToLower(strings.TrimSpace(host))

		if exactMatch != "" && exactMatch == host {
			// good match
			return nil
		}

		// try replacing labels in the name with
		// wildcards until we get a match
		labels := strings.Split(host, ".")
		for i := range labels {
			labels[i] = "*"
			candidate := strings.Join(labels, ".")
			if wildcard == candidate {
				// good match
				return nil
			}
		}

		return fmt.Errorf("ong/server: host %q not configured in HostWhitelist", host)
	}
}

func cleanDomain(domain string) string {
	d := strings.ReplaceAll(domain, "*", "")
	d = strings.TrimLeft(d, ".")
	return d
}
