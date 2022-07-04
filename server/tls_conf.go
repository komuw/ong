package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"

	"golang.org/x/net/idna"

	ongErrors "github.com/komuw/ong/errors"
	"github.com/komuw/ong/log"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

// Most of the code here is insipired(or taken from) by:
//   (a) https://github.com/eliben/code-for-blog whose license(Unlicense) can be found here:                                   https://github.com/eliben/code-for-blog/blob/464a32f686d7646ba3fc612c19dbb550ec8a05b1/LICENSE
//   (b) https://github.com/FiloSottile/mkcert   whose license(BSD 3-Clause ) can be found here:                               https://github.com/FiloSottile/mkcert/blob/v1.4.4/LICENSE
//   (c) https://github.com/golang/crypto/blob/master/acme/autocert/autocert.go whose license(BSD 3-Clause) can be found here: https://github.com/golang/crypto/blob/05595931fe9d3f8894ab063e1981d28e9873e2cb/LICENSE
//   (d) https://github.com/caddyserver/certmagic/blob/master/handshake.go whose license(Apache 2.0) can be found here:        https://github.com/caddyserver/certmagic/blob/v0.16.1/LICENSE.txt
//

// getTlsConfig returns a proper tls configuration given the options passed in.
// The tls config may either procure certifiates from LetsEncrypt, from disk or be nil(for non-tls traffic)
func getTlsConfig(o opts, logger log.Logger) (*tls.Config, error) {
	if o.tls.enabled {
		if err := validateDomain(o.tls.domain); err != nil {
			return nil, err
		}
	}

	if o.tls.email != "" {
		// 1. use letsencrypt.
		//
		const letsEncryptProductionUrl = "https://acme-v02.api.letsencrypt.org/directory"
		_ = letsEncryptProductionUrl
		const letsEncryptStagingUrl = "https://acme-staging-v02.api.letsencrypt.org/directory"
		m := &autocert.Manager{
			Client:     &acme.Client{DirectoryURL: letsEncryptStagingUrl},
			Cache:      autocert.DirCache("ong-certifiate-dir"),
			Prompt:     autocert.AcceptTOS,
			Email:      o.tls.email,
			HostPolicy: customHostWhitelist(o.tls.domain),
		}
		tlsConf := &tls.Config{
			// taken from:
			// https://github.com/golang/crypto/blob/05595931fe9d3f8894ab063e1981d28e9873e2cb/acme/autocert/autocert.go#L228-L234
			NextProtos: []string{
				"h2", // enable HTTP/2
				"http/1.1",
				acme.ALPNProto, // enable tls-alpn ACME challenges
			},
			GetCertificate: func(info *tls.ClientHelloInfo) (certificate *tls.Certificate, e error) {
				return m.GetCertificate(info)
			},
		}
		return tlsConf, nil
	}
	if o.tls.certFile != "" {
		// 2. get from disk.
		//
		if len(o.tls.keyFile) < 1 {
			return nil, ongErrors.New("keyFile cannot be empty if certFile is also specified")
		}
		c, err := tls.LoadX509KeyPair(o.tls.certFile, o.tls.keyFile)
		if err != nil {
			err = ongErrors.Wrap(err)
			return nil, err
		}

		tlsConf := &tls.Config{
			NextProtos: []string{
				"h2", // enable HTTP/2
				"http/1.1",
				acme.ALPNProto, // enable tls-alpn ACME challenges
			},
			GetCertificate: func(info *tls.ClientHelloInfo) (certificate *tls.Certificate, e error) {
				// GetCertificate returns a Certificate based on the given ClientHelloInfo.
				// it is called if `tls.Config.Certificates` is empty.
				//
				return &c, nil
			},
		}
		return tlsConf, nil
	}

	// 3. non-tls traffic.
	return nil, nil
}

func validateDomain(domain string) error {
	if len(domain) < 1 {
		return ongErrors.New("domain cannot be empty if email/certFile is also specified")
	}
	if strings.Count(domain, "*") > 1 {
		return ongErrors.New("domain can only contain one wildcard character")
	}
	if strings.Contains(domain, "*") && !strings.HasPrefix(domain, "*") {
		return ongErrors.New("wildcard character should be a prefix")
	}
	if strings.Contains(domain, "*") && domain[1] != '.' {
		return ongErrors.New("wildcard character should be followed by a `.` character")
	}

	if !strings.Contains(domain, "*") {
		// not wildcard
		if _, err := idna.Lookup.ToASCII(domain); err != nil {
			return ongErrors.Wrap(err)
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

		return ongErrors.New(fmt.Sprintf("ong/acme: host %q not configured in HostWhitelist", host))
	}
}

func cleanDomain(domain string) string {
	d := strings.ReplaceAll(domain, "*", "")
	d = strings.TrimLeft(d, ".")
	return d
}
