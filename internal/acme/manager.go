// Package acme provides automatic access to certificates from ACME-based certificate authorities(like Let's Encrypt).
package acme

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"golang.org/x/exp/slog"
	"golang.org/x/net/idna"
)

const (
	accountKeyFileName = "ong_acme_account_private.key"
	certFileName       = "ong_acme_certificate.crt"
	certKeyFileName    = "ong_acme_certificate.key"
	tokenFileName      = "ong_acme_certificate.token"

	// With HTTP validation, the client in an ACME transaction proves its
	// control over a domain name by proving that it can provision HTTP
	// resources on a server accessible under that domain name.
	// The path at which the resource is provisioned is comprised of the
	// fixed prefix "/.well-known/acme-challenge/", followed by the "token" value in the challenge.
	// https://datatracker.ietf.org/doc/html/rfc8555#section-8.3
	challengeURI = "/.well-known/acme-challenge/"
)

// hostPolicy specifies which host names the Manager is allowed to respond to.
// It returns a non-nil error if the host should be rejected.
// The returned error is accessible via tls.Conn.Handshake and its callers.
// See Manager's hostPolicy field and GetCertificate method docs for more details.
type hostPolicy func(ctx context.Context, host string) error

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

// GetCertificate returns a function that implements [tls.Config.GetCertificate].
// It provides a TLS certificate for hello.ServerName host.
//
// GetCertificate panics on error, however the returned function handles errors normally.
func GetCertificate(domain, email, acmeDirectoryUrl string, l *slog.Logger) func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	if err := Validate(domain); err != nil {
		panic(err)
	}
	man := initManager(domain, email, acmeDirectoryUrl, l)

	return func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		name := hello.ServerName

		if name == "" {
			return nil, errors.New("ong/acme: missing server name")
		}
		if !strings.Contains(strings.Trim(name, "."), ".") {
			return nil, errors.New("ong/acme: server name component count invalid")
		}

		// Some server names in the handshakes started by some clients (such as cURL) are not converted to Punycode, which will
		// prevent us from obtaining certificates for them.
		// https://github.com/golang/crypto/blob/v0.10.0/acme/autocert/autocert.go#L249-L273
		name, err := idna.Lookup.ToASCII(name)
		if err != nil {
			return nil, errors.New("ong/acme: server name contains invalid character")
		}

		// see: golang.org/issue/18114
		dmn := strings.TrimSuffix(name, ".")

		return man.getCert(dmn)
	}
}

// Handler returns a [http.Handler] that can be used to respond to ACME "http-01" challenge responses.
// Ong configures this for you automatically, so users of Ong do not have to worry about this handler.
//
// Handler panics on error, however the returned [http.Handler] handles errors normally.
func Handler(wrappedHandler http.Handler) http.HandlerFunc {
	// Support for acme certificate manager needs to be added in three places:
	// (a) In http middlewares.
	// (b) In http server.
	// (c) In http multiplexer.

	// With HTTP validation, the client in an ACME transaction proves its
	// control over a domain name by proving that it can provision HTTP
	// resources on a server accessible under that domain name.
	// The path at which the resource is provisioned is comprised of the
	// fixed prefix "/.well-known/acme-challenge/", followed by the "token" value in the challenge.
	// https://datatracker.ietf.org/doc/html/rfc8555#section-8.3

	diskCacheDir, err := diskCachedir()
	if err != nil {
		panic(err)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		// This code is taken from; https://github.com/golang/crypto/blob/v0.10.0/acme/autocert/autocert.go#L398-L401
		if strings.HasPrefix(r.URL.Path, challengeURI) {
			domain := r.Host

			{ // error cases
				isTls := strings.EqualFold(r.URL.Scheme, "https") || r.TLS != nil
				if isTls {
					// Acme requests should be HTTP(not https), though I don't know if it is mandated.
					// Spec says:
					//   "The server verifies the client's control of the domain by verifying ..."
					//   "Construct a URL by populating the URL `http://{domain}/.well-known/acme-challenge/{token}`"
					// https://datatracker.ietf.org/doc/html/rfc8555#section-8.3
					// So the doc uses `http` not `https` though I don't know if it is a mandate.
					wrappedHandler.ServeHTTP(w, r)
					return
				}

				if len(domain) < 2 || unicode.IsDigit(rune(domain[0])) { // It is a bare IP.
					wrappedHandler.ServeHTTP(w, r)
					return
				}

				if strings.Contains(domain, ":") {
					d, _, errA := net.SplitHostPort(domain)
					if errA != nil {
						wrappedHandler.ServeHTTP(w, r)
						return
					}

					domain = d
				}
			}

			certPath := filepath.Join(diskCacheDir, domain, tokenFileName)
			tok, errC := os.ReadFile(certPath)
			if errC != nil {
				http.Error(
					w,
					errC.Error(),
					http.StatusInternalServerError,
				)
				return
			}

			_, _ = fmt.Fprint(w, string(tok))
			w.WriteHeader(http.StatusOK)

			return
		}

		wrappedHandler.ServeHTTP(w, r)
	}
}

// manager manages the TLS certificate request process.
// Its main method is [manager.getCert]
type manager struct {
	// Note that we do not need to build automated certificate renewal.
	// Certificates get renewed on the fly.
	// Whenever we fetch certs from memory/disk, we check if they are expired.
	// See: [certIsValid]
	// +checklocks:mu
	cache            *certCache
	websiteDomain    string
	email            string
	acmeDirectoryUrl string
	diskCacheDir     string
	// +checklocks:mu
	hp hostPolicy
	l  *slog.Logger

	// mu protects access to;
	// - In memory cache.
	// - Disk access.
	// - ACME server.
	//
	// If we get 1_000 requests for a domain whose cert we do not have in memory,
	// we would send 1_000 requests to ACME without this mutex.
	// NB: This mutex is for all domains even unrelated ones.
	//     Ideally, it should be a mutex per domain.
	mu sync.Mutex
}

// initManager is only used in tests. Use [GetCertificate] instead.
//
// The optional argument testDiskCache is only used for internal test purposes.
// It panics on error.
func initManager(domain, email, acmeDirectoryUrl string, l *slog.Logger, testDiskCache ...string) *manager {
	diskCacheDir := ""

	if len(testDiskCache) > 0 {
		// allow for tests.
		// todo: check if `testing.Testing()` and panic if `testDiskCache` is there and it is not testing.
		diskCacheDir = testDiskCache[0]
	} else {
		d, errA := diskCachedir()
		if errA != nil {
			panic(errA)
		}
		diskCacheDir = d
	}

	c := newCache()

	{ // populate cache.
		if files, errB := os.ReadDir(diskCacheDir); errB == nil {
			for _, f := range files {
				// layout is like:
				// diskCacheDir/
				//   domainName/
				//     ong_acme_certificate.crt
				if f.IsDir() {
					dmn := f.Name()
					certPath := filepath.Join(diskCacheDir, dmn, certFileName)
					cert, errC := certFromDisk(certPath)
					if errC != nil {
						continue
					}
					c.setCert(dmn, cert)
				}
			}
		}
	}

	return &manager{
		cache:            c,
		websiteDomain:    domain,
		email:            email,
		acmeDirectoryUrl: acmeDirectoryUrl,
		diskCacheDir:     diskCacheDir,
		hp:               customHostWhitelist(domain),
		l:                l,
	}
}

// getCert fetches a tls certificate for domain.
func (m *manager) getCert(domain string) (cert *tls.Certificate, _ error) {
	/*
		1. Get cert from memory/cache.
		2. Else get from disk(also save to memory).
		3. Else get from ACME(also save to disk and memory).
	*/
	m.mu.Lock()
	defer m.mu.Unlock()
	// The defer for mutex unlock will happen after the defer for adding certs to cache/disk.
	// https://go.dev/play/p/tQ7JOiFcCLf

	// todo: add context cancellation.
	// see; crypto/acme/autocert

	certFromAcme := false
	defer func() {
		// 4. Add to cache and disk.
		if cert != nil && certFromAcme {
			m.cache.setCert(domain, cert)
			if errA := m.toDisk(domain, cert); errA != nil {
				m.l.Error("m.toDisk", "error", errA)
			}
		}
		// We do not need to log the `getCert()` return error.
		// This is because the http.Server will do that.
	}()

	{ // 1. Get from cache.
		c, _ := m.cache.getCert(domain)
		if c != nil {
			return c, nil
		}
	}

	{ // 2. Get from disk.
		c, _ := m.fromDisk(domain)
		if c != nil {
			return c, nil
		}
	}

	{ // 3. Get from ACME.
		c, errB := m.fromAcme(domain)
		if errB != nil {
			return nil, errB
		}

		if errC := m.hp(context.Background(), domain); errC != nil {
			return nil, errC
		}

		certFromAcme = true

		return c, nil
	}
}

func (m *manager) fromDisk(domain string) (*tls.Certificate, error) {
	// see: https://github.com/golang/crypto/blob/v0.10.0/acme/autocert/autocert.go#L470-L472

	certPath := filepath.Join(m.diskCacheDir, domain, certFileName)
	cert, err := certFromDisk(certPath)
	if err != nil {
		return nil, err
	}

	return cert, nil
}

func (m *manager) toDisk(domain string, cert *tls.Certificate) error {
	// see: https://github.com/golang/crypto/blob/v0.10.0/acme/autocert/autocert.go#L519

	certPath := filepath.Join(m.diskCacheDir, domain, certFileName)

	return certToDisk(cert, certPath)
}

// fromAcme gets a certificate for domain from an ACME server.
func (m *manager) fromAcme(domain string) (_ *tls.Certificate, acmeError error) {
	var (
		directoryResponse     directory
		actResponse           account
		orderResponse         order
		authorizationResponse authorization
		token                 string
		updatedOrder          order
	)

	defer func() {
		if acmeError != nil {
			m.l.Error("m.fromAcme",
				"directoryResponse", directoryResponse,
				"actResponse", actResponse,
				"orderResponse", orderResponse,
				"authorizationResponse", authorizationResponse,
				"token", token,
				"updatedOrder", updatedOrder,
				"error", acmeError,
			)
		}
	}()

	email := m.email
	accountKeyPath := filepath.Join(m.diskCacheDir, accountKeyFileName)

	accountPrivKey, errA := getEcdsaPrivKey(accountKeyPath)
	if errA != nil {
		return nil, errA
	}

	certKeyPath := filepath.Join(m.diskCacheDir, domain, certKeyFileName)
	certPrivKey, errB := getEcdsaPrivKey(certKeyPath)
	if errB != nil {
		return nil, errB
	}

	directoryResponse, errC := getDirectory(m.acmeDirectoryUrl, m.l)
	if errC != nil {
		return nil, errC
	}

	actResponse, errD := getAccount(directoryResponse.NewAccountURL, directoryResponse.NewNonceURL, email, accountPrivKey, m.l)
	if errD != nil {
		return nil, errD
	}

	domains := []string{domain}
	orderResponse, errE := submitOrder(directoryResponse.NewOrderURL, directoryResponse.NewNonceURL, actResponse.kid, domains, accountPrivKey, m.l)
	if errE != nil {
		return nil, errE
	}

	authorizationURLS := orderResponse.Authorizations
	authorizationResponse, errF := fetchChallenges(authorizationURLS, directoryResponse.NewNonceURL, actResponse.kid, accountPrivKey, m.l)
	if errF != nil {
		return nil, errF
	}

	token, errG := jWKThumbprint(accountPrivKey.PublicKey, authorizationResponse.EffectiveChallenge.Token)
	if errG != nil {
		return nil, errG
	}
	m.setToken(domain, token)

	if _, errH := respondToChallenge(authorizationResponse.EffectiveChallenge, directoryResponse.NewNonceURL, actResponse.kid, accountPrivKey, m.l); errH != nil {
		return nil, errH
	}

	updatedOrder, errI := sendCSR(domain, orderResponse, directoryResponse.NewNonceURL, actResponse.kid, accountPrivKey, certPrivKey, m.l)
	if errI != nil {
		return nil, errI
	}

	certBytes, errJ := downloadCertificate(updatedOrder, directoryResponse.NewNonceURL, actResponse.kid, accountPrivKey, m.l)
	if errJ != nil {
		return nil, errJ
	}

	buf := &bytes.Buffer{}
	if errK := encodeECDSAKey(buf, certPrivKey); errK != nil {
		return nil, errK
	}
	if _, errL := buf.Write(certBytes); errL != nil {
		return nil, errL
	}

	cert, errM := certFromReader(buf)
	return cert, errM
}

func (m *manager) setToken(domain, token string) {
	certPath := filepath.Join(m.diskCacheDir, domain, tokenFileName)
	if err := os.WriteFile(certPath, []byte(token), 0o600); err != nil {
		m.l.Error("m.setToken", "error", err)
	}
}

// certCache is an in memory cache for certificates and also ACME http-01 challenge tokens
type certCache struct {
	// The certs map should be accesed via a mutex.
	// See the mutex inside [manager].
	certs map[string]*tls.Certificate
}

func newCache() *certCache {
	return &certCache{
		certs: map[string]*tls.Certificate{},
	}
}

func (c *certCache) getCert(domain string) (*tls.Certificate, error) {
	if cert, ok := c.certs[domain]; ok && certIsValid(cert) {
		return cert, nil
	}

	return nil, errors.New("ong/acme: cache does not have certificate")
}

func (c *certCache) setCert(domain string, cert *tls.Certificate) {
	c.certs[domain] = cert
}
