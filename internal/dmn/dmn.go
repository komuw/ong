// Package dmn contains domain utilities used by multiple ong packages.
package dmn

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

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

var cm = &cManager{cm: map[string]*autocert.Manager{}}

type cManager struct {
	mu sync.Mutex // protects cm
	cm map[string]*autocert.Manager
}

func (c *cManager) get(domain string) (*autocert.Manager, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	r, ok := c.cm[domain]

	return r, ok
}

func (c *cManager) set(domain string, m *autocert.Manager) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cm[domain] = m
}

// CertManager returns an ACME certificate manager for the given domain.
// This should be called with a valid domain. Call [Validate] before calling this.
// Callers should check if return value is nil.
func CertManager(domain, acmeEmail, acmeDirectoryUrl string) *autocert.Manager {
	if domain == "" || acmeEmail == "" || acmeDirectoryUrl == "" {
		return nil
	}

	if m, ok := cm.get(domain); ok {
		return m
	}

	m := &autocert.Manager{
		Client: &acme.Client{
			DirectoryURL: acmeDirectoryUrl,
			HTTPClient: &http.Client{
				Timeout: 13 * time.Second,
			},
		},
		Cache:      autocert.DirCache("ong-certifiate-dir"),
		Prompt:     autocert.AcceptTOS,
		Email:      acmeEmail,
		HostPolicy: customHostWhitelist(domain),
	}
	cm.set(domain, m)

	return m
}

// Validate checks domain for validity.
// domain is the domain name of your website. It can be an exact domain, subdomain or wildcard.
func Validate(domain string) error {
	if len(domain) < 1 {
		return errors.New("ong: domain cannot be empty")
	}
	if strings.Count(domain, "*") > 1 {
		return errors.New("ong: domain can only contain one wildcard character")
	}
	if strings.Contains(domain, "*") && !strings.HasPrefix(domain, "*") {
		return errors.New("ong: domain wildcard character should be a prefix")
	}
	if strings.Contains(domain, "*") && domain[1] != '.' {
		return errors.New("ong: domain wildcard character should be followed by a `.` character")
	}

	toCheck := domain
	if strings.Contains(domain, "*") {
		// remove the `*` and `.`
		toCheck = domain[2:]
	}

	if _, err := idna.Registration.ToASCII(toCheck); err != nil {
		return fmt.Errorf("ong: domain is invalid: %w", err)
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
