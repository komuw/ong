// Package acme provides automatic access to certificates from ACME-based certificate authorities(like Let's Encrypt).
package acme

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/idna"
)

// Some of the code here is inspired(or taken from) by:
//   (a) https://github.com/golang/crypto/blob/master/acme/autocert/autocert.go whose license(BSD 3-Clause) can be found here: https://github.com/golang/crypto/blob/v0.18.0/LICENSE
//   (b) https://github.com/komuw/sewer whose license(MIT) can be found here:                                                  https://github.com/komuw/sewer/blob/0.8.0/LICENSE.txt
//

const (
	accountKeyFileName = "ong_acme_account_private.key"
	certAndKeyFileName = "ong_acme_certificate_and_key.crt" // has key and cert concatenated.
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

	if _, err := idna.Lookup.ToASCII(toCheck); err != nil {
		// We use `idna.Lookup` instead of `idna.Registration` because `ong` is only
		// involved with domain lookups. `ong` is not a domain registrar.
		return fmt.Errorf("ong: domain is invalid: %w", err)
	}

	return nil
}

// GetCertificate returns a function that implements [tls.Config.GetCertificate].
// It provides a TLS certificate for hello.ServerName host.
// It should be called once and then the returned function can be called per request.
//
// GetCertificate panics on error, however the returned function handles errors normally.
func GetCertificate(tlsHosts []string, email, acmeDirectoryUrl string, l *slog.Logger) func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	man, err := initManager(tlsHosts, email, acmeDirectoryUrl, l)
	if err != nil {
		panic(err)
	}

	return func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		name := hello.ServerName

		if c := man.getCertFastPath(name); c != nil {
			return c, nil
		}

		if name == "" {
			return nil, errors.New("ong/acme: missing server name")
		}
		if !strings.Contains(strings.Trim(name, "."), ".") {
			return nil, errors.New("ong/acme: server name component count invalid")
		}

		// Some server names in the handshakes started by some clients (such as cURL) are not converted to Punycode, which will
		// prevent us from obtaining certificates for them.
		// https://github.com/golang/crypto/blob/v0.10.0/acme/autocert/autocert.go#L249-L273
		name, errA := idna.Lookup.ToASCII(name)
		if errA != nil {
			return nil, errors.New("ong/acme: server name contains invalid character")
		}

		// see: golang.org/issue/18114
		dmn := strings.TrimSuffix(name, ".")

		if _, errB := netip.ParseAddr(dmn); errB == nil {
			// It is an IP address.
			// See: https://github.com/komuw/ong/issues/305
			return nil, errors.New("ong/acme: cannot issue a certificate for an IP  address")
		}

		ctx, cancel := context.WithTimeout(
			// We do not use `hello.Context()` here because, even if the tls hello is canceled,
			// we would like to continue with certificate procurement.
			// Since cert will end up been cached in memory, it is not wasteful to continue after tls hello has ended.
			context.Background(),
			// The timeout needs to be long enough to last the whole process of fetching
			// certificates from ACME servers including all the challenge-request-wait-response flow.
			// There are about 10 http calls to ACME, each of which in turn calls [getNonce] at least once.
			// Thus total of about 20.
			// If each call has a worst case of 15seconds, we get total duration of 300secs(5minutes)
			5*time.Minute,
		)
		defer cancel()

		return man.getCert(ctx, dmn)
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

	diskCacheDir, errA := diskCachedir()
	if errA != nil {
		panic(errA)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		// This code is taken from; https://github.com/golang/crypto/blob/v0.10.0/acme/autocert/autocert.go#L398-L401
		if strings.HasPrefix(r.URL.Path, challengeURI) {
			fileTok := []byte("ong/acme: placeholder token") // If you see this as a response, consider it a bug in ong.
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
					// Also see: https://letsencrypt.org/docs/challenge-types/
					//
					errB := errors.New(
						// `ong/acme` only accepts `http-01` challenge, it does not accept `tls-alpn-01`.
						"ong/acme: well-known/acme-challenge request should not be https",
					)
					http.Error(
						w,
						errB.Error(),
						http.StatusTeapot,
					)
					return
				}

				if strings.Contains(domain, ":") {
					d, _, errD := net.SplitHostPort(domain)
					if errD != nil {
						http.Error(
							w,
							errD.Error(),
							http.StatusTeapot,
						)
						return
					}

					domain = d
				}

				if _, errC := netip.ParseAddr(domain); errC == nil {
					e := errors.New("ong/acme: request.host for well-known/acme-challenge request should not be IP address")
					http.Error(
						w,
						e.Error(),
						http.StatusTeapot,
					)
					return
				}

				tokenPath := filepath.Join(diskCacheDir, domain, tokenFileName)
				tok, errE := os.ReadFile(tokenPath)
				if errE != nil {
					http.Error(
						w,
						errE.Error(),
						http.StatusInternalServerError,
					)
					return
				}
				fileTok = tok

				{ // Check if the token supplied in the url is known to us and error otherwise.
					iTok := strings.Split(
						strings.TrimRight(r.URL.Path, "/"),
						"/",
					)
					incomingTok := iTok[len(iTok)-1]

					if string(fileTok) != incomingTok { // This assumes that the code that wrote `tokenFileName` to disk didn't add any characters like newlines.
						e := errors.New("ong/acme: requested token not found for well-known/acme-challenge")
						http.Error(
							w,
							e.Error(),
							http.StatusNotFound,
						)
						return
					}
				}
			}

			_, _ = w.Write(fileTok)
			w.WriteHeader(http.StatusOK)
			return
		}

		wrappedHandler.ServeHTTP(w, r)
	}
}

// hostPolicy specifies which host names the Manager is allowed to respond to.
// It returns a non-nil error if the host should be rejected.
// The returned error is accessible via tls.Conn.Handshake and its callers.
// See Manager's hostPolicy field and GetCertificate method docs for more details.
type hostPolicy func(host string) error

// manager manages the TLS certificate request process.
// Its main method is [manager.getCert]
type manager struct {
	// Note that we do not need to build automated certificate renewal.
	// Certificates get renewed on the fly.
	// Whenever we fetch certs from memory/disk, we check if they are expired.
	// See: [certIsValid]
	// +checklocks:mu
	cache            *certCache
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
// The optional argument testDiskCache is only used for internal test purposes.
func initManager(tlsHosts []string, email, acmeDirectoryUrl string, l *slog.Logger, testDiskCache ...string) (*manager, error) {
	diskCacheDir := ""

	if len(testDiskCache) > 0 && !testing.Testing() {
		return nil, errors.New("ong/acme: optional argument testDiskCache should only be used for internal test purposes")
	}

	if len(testDiskCache) > 0 && testing.Testing() {
		diskCacheDir = testDiskCache[0]
	} else {
		d, errA := diskCachedir()
		if errA != nil {
			return nil, errA
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
				//     ong_acme_certificate_and_key.crt
				if f.IsDir() {
					dmn := f.Name()
					certPath := filepath.Join(diskCacheDir, dmn, certAndKeyFileName)
					cert, errC := certFromDisk(certPath)
					if errC != nil {
						continue
					}
					c.setCert(dmn, cert)
				}
			}
		}
	}

	hp, err := hostWhitelist(tlsHosts...)
	if err != nil {
		return nil, err
	}

	return &manager{
		cache:            c,
		email:            email,
		acmeDirectoryUrl: acmeDirectoryUrl,
		diskCacheDir:     diskCacheDir,
		hp:               hp,
		l:                l,
	}, nil
}

// getCertFastPath fetches a tls certificate for domain from memory only.
// It returns nil if certificate was not found. Callers should check if certificate is nil.
//
// Note that this cannot be called inside getCert since both of them take the same lock.
func (m *manager) getCertFastPath(domain string) *tls.Certificate {
	m.mu.Lock()
	defer m.mu.Unlock()

	c, _ := m.cache.getCert(domain)
	return c
}

// getCert fetches a tls certificate for domain from memory/disk/acme.
func (m *manager) getCert(ctx context.Context, domain string) (cert *tls.Certificate, _ error) {
	/*
		1. Get cert from memory/cache.
		2. Else get from disk(also save to memory).
		3. Else get from ACME(also save to disk and memory).
	*/
	m.mu.Lock()
	defer m.mu.Unlock()

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
		if errA := m.hp(domain); errA != nil {
			// We should check hostPolicy before calling ACME to minimize wastage of compute.
			return nil, errA
		}

		c, errB := m.fromAcme(ctx, domain)
		if errB != nil {
			return nil, errB
		}

		cert = c
	}

	{ // 4. Add to cache and disk. This should be done ONLY if cert came from ACME.
		// This block should ideally have been in a defer;
		// However; the `checklocks` static analyzer complains.
		// see: https://github.com/komuw/ong/blob/3153948e1a6ac10c7744ed46356cd1491f1dda50/internal/acme/acme.go#L284-L295
		//      https://github.com/komuw/ong/issues/297
		if cert != nil {
			m.cache.setCert(domain, cert)
			if errC := m.toDisk(domain, cert); errC != nil {
				m.l.Error("m.toDisk", "error", errC)
			}
		}
		// We do not need to log the `getCert()` return error.
		// This is because the http.Server will do that.
	}

	return cert, nil
}

func (m *manager) fromDisk(domain string) (*tls.Certificate, error) {
	// see: https://github.com/golang/crypto/blob/v0.10.0/acme/autocert/autocert.go#L470-L472

	certPath := filepath.Join(m.diskCacheDir, domain, certAndKeyFileName)
	cert, err := certFromDisk(certPath)
	if err != nil {
		return nil, err
	}

	return cert, nil
}

func (m *manager) toDisk(domain string, cert *tls.Certificate) error {
	// see: https://github.com/golang/crypto/blob/v0.10.0/acme/autocert/autocert.go#L519

	certPath := filepath.Join(m.diskCacheDir, domain, certAndKeyFileName)

	return certToDisk(cert, certPath)
}

// fromAcme gets a certificate for domain from an ACME server.
func (m *manager) fromAcme(ctx context.Context, domain string) (_ *tls.Certificate, acmeError error) {
	var (
		directoryResponse         directory
		actResponse               account
		orderResponse             order
		authorizationResponse     authorization
		updatedEffectiveChallenge challenge
		token                     string
		updatedOrder              order
	)

	defer func() {
		if acmeError != nil {
			m.l.Error("m.fromAcme",
				"directoryResponse", directoryResponse,
				"actResponse", actResponse,
				"orderResponse", orderResponse,
				"authorizationResponse", authorizationResponse,
				"updatedEffectiveChallenge", updatedEffectiveChallenge,
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

	directoryResponse, errC := getDirectory(ctx, m.acmeDirectoryUrl, m.l)
	if errC != nil {
		return nil, errC
	}

	actResponse, errD := getAccount(ctx, directoryResponse.NewAccountURL, directoryResponse.NewNonceURL, email, accountPrivKey, m.l)
	if errD != nil {
		return nil, errD
	}

	domains := []string{domain}
	orderResponse, errE := submitOrder(ctx, directoryResponse.NewOrderURL, directoryResponse.NewNonceURL, actResponse.kid, domains, accountPrivKey, m.l)
	if errE != nil {
		return nil, errE
	}

	authorizationURLS := orderResponse.Authorizations
	authorizationResponse, errF := fetchChallenges(ctx, authorizationURLS, directoryResponse.NewNonceURL, actResponse.kid, accountPrivKey, m.l)
	if errF != nil {
		return nil, errF
	}

	token, errG := jWKThumbprint(accountPrivKey.PublicKey, authorizationResponse.EffectiveChallenge.Token)
	if errG != nil {
		return nil, errG
	}
	m.setToken(domain, token)

	updatedEffectiveChallenge, errH := respondToChallenge(ctx, authorizationResponse.EffectiveChallenge, directoryResponse.NewNonceURL, actResponse.kid, accountPrivKey, m.l)
	if errH != nil {
		return nil, errH
	}

	updatedOrder, errI := sendCSR(ctx, domain, orderResponse, directoryResponse.NewNonceURL, actResponse.kid, accountPrivKey, certPrivKey, m.l)
	if errI != nil {
		return nil, errI
	}

	certBytes, errJ := downloadCertificate(ctx, updatedOrder, directoryResponse.NewNonceURL, actResponse.kid, accountPrivKey, m.l)
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
	tokenPath := filepath.Join(m.diskCacheDir, domain, tokenFileName)
	if err := os.WriteFile(tokenPath, []byte(token), 0o600); err != nil {
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
