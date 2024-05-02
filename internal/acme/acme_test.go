package acme

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	mathRand "math/rand/v2"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.akshayshah.org/attest"
)

// getDomain returns a valid unique domain.
func getDomain() string {
	r := mathRand.IntN(100_000) + 1
	return fmt.Sprintf("some-sample-%d-domain.com", r)
}

// createX509Cert is used in tests.
func createX509Cert(t testing.TB, domain string, privKey *ecdsa.PrivateKey) []byte {
	t.Helper()

	pubKey := privKey.Public()

	randomSerialNumber := func() *big.Int {
		serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
		serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
		attest.Ok(t, err)
		return serialNumber
	}

	certTemplate := x509.Certificate{
		SerialNumber: randomSerialNumber(),
		Subject: pkix.Name{
			Organization: []string{"Ong ACME tests."},
		},
		DNSNames:  []string{domain},
		NotBefore: time.Now().UTC(),
		NotAfter:  time.Now().UTC().Add(5 * 24 * time.Hour), // 5days
		KeyUsage:  x509.KeyUsageDigitalSignature,
	}

	certDer, err := x509.CreateCertificate(rand.Reader, &certTemplate, &certTemplate, pubKey, privKey)
	attest.Ok(t, err)

	return certDer
}

// createTlsCert is used in tests.
func createTlsCert(t testing.TB, domain string) *tls.Certificate {
	t.Helper()

	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	attest.Ok(t, err)

	certDer := createX509Cert(t, domain, privKey)

	tmp := t.TempDir()
	certPath := filepath.Join(tmp, "cert.crt")
	keyPath := filepath.Join(tmp, "key.key")

	w, err := os.Create(certPath)
	attest.Ok(t, err)

	err = pem.Encode(w, &pem.Block{Type: "CERTIFICATE", Bytes: certDer})
	attest.Ok(t, err)

	privBytes, err := x509.MarshalPKCS8PrivateKey(privKey)
	attest.Ok(t, err)
	w, err = os.Create(keyPath)
	attest.Ok(t, err)
	err = pem.Encode(w, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})
	attest.Ok(t, err)

	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	attest.Ok(t, err)

	return &cert
}

func TestValidateDomain(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		tt := []struct {
			domain       string
			shouldSucced bool
		}{
			{"example.com", true},
			{"example.org", true},
			{"xn--9caa.com", true}, // éé.com
			{"one.example.com", true},
			//
			{"*.example.org", true},
			{"*example.org", false}, // wildcard character should be followed by a `.` character
			{"*.example.*", false},
			{"example.*org", false},
			//
			{"", false},
			{"exampl_e.com", false}, // underscore(rune U+005F) is disallowed in domain names.
			//
			{"dummy", true},
			{getDomain(), true},
		}
		for _, test := range tt {
			err := Validate(test.domain)
			if test.shouldSucced {
				attest.Ok(t, err, attest.Sprintf("failed: %s ", test.domain))
			} else {
				attest.NotZero(t, err, attest.Sprintf("failed: %s ", test.domain))
			}
		}
	})
}

func TestManager(t *testing.T) {
	t.Parallel()

	email := "hey+sample@gmail.com"
	acmeDirectoryUrl := "https://some-domain.com/directory"

	l := slog.Default()

	t.Run("GetCertificate", func(t *testing.T) {
		t.Parallel()

		domain := getDomain()
		diskCacheDir, errA := diskCachedir()
		attest.Ok(t, errA)
		{ // prep by saving a certificate for the domain to disk.
			certPath := filepath.Join(diskCacheDir, domain, certAndKeyFileName)
			cert := createTlsCert(t, domain)
			errB := certToDisk(cert, certPath)
			attest.Ok(t, errB)
		}

		getCrt := GetCertificate([]string{domain}, email, acmeDirectoryUrl, l)
		cert, errC := getCrt(&tls.ClientHelloInfo{
			ServerName: domain,
		})
		attest.Ok(t, errC)
		attest.NotZero(t, cert)
		attest.True(t, certIsValid(cert))
	})

	t.Run("initManager", func(t *testing.T) {
		t.Parallel()

		domain := getDomain()
		testDiskCache := t.TempDir()
		m, err := initManager([]string{domain}, email, acmeDirectoryUrl, l, testDiskCache)
		attest.Ok(t, err)
		attest.NotZero(t, m)
		attest.NotZero(t, m.cache)
		attest.NotZero(t, m.email)
		attest.NotZero(t, m.diskCacheDir)
		attest.Equal(t, m.diskCacheDir, testDiskCache)

		attest.Equal(t, len(m.cache.certs), 0)
	})

	t.Run("manager fills from disk", func(t *testing.T) {
		t.Parallel()

		testDiskCache := t.TempDir()
		domain := getDomain()
		{ // prep by saving a certificate for the domain to disk.
			certPath := filepath.Join(testDiskCache, domain, certAndKeyFileName)
			cert := createTlsCert(t, domain)
			err := certToDisk(cert, certPath)
			attest.Ok(t, err)
		}

		m, err := initManager([]string{domain}, email, acmeDirectoryUrl, l, testDiskCache)
		attest.Ok(t, err)
		attest.NotZero(t, m)
		attest.Equal(t, len(m.cache.certs), 1)
	})

	t.Run("getCert", func(t *testing.T) {
		t.Parallel()

		testDiskCache := t.TempDir()
		domain := getDomain()
		{ // prep by saving a certificate for the domain to disk.
			certPath := filepath.Join(testDiskCache, domain, certAndKeyFileName)
			cert := createTlsCert(t, domain)
			err := certToDisk(cert, certPath)
			attest.Ok(t, err)
		}

		m, err := initManager([]string{domain}, email, acmeDirectoryUrl, l, testDiskCache)
		attest.Ok(t, err)
		attest.NotZero(t, m)

		cert, err := m.getCert(context.Background(), domain)
		attest.Ok(t, err)
		attest.NotZero(t, cert)
	})

	t.Run("getCert bad domain", func(t *testing.T) {
		t.Parallel()

		testDiskCache := t.TempDir()
		domain := getDomain()

		ts := httptest.NewServer(someAcmeServerHandler(t, domain))
		t.Cleanup(func() {
			ts.Close()
		})

		acmeDirUrl, errA := url.JoinPath(ts.URL, "/directory")
		attest.Ok(t, errA)

		m, err := initManager([]string{domain}, email, acmeDirUrl, l, testDiskCache)
		attest.Ok(t, err)
		attest.NotZero(t, m)

		cert, err := m.getCert(context.Background(), "cloudflare.com")
		attest.Zero(t, cert)
		attest.NotZero(t, err)
		attest.Subsequence(t, err.Error(), "not configured in HostWhitelist")
	})

	t.Run("getCertFastPath", func(t *testing.T) {
		t.Parallel()

		testDiskCache := t.TempDir()
		domain := getDomain()
		{ // prep by saving a certificate for the domain to disk.
			certPath := filepath.Join(testDiskCache, domain, certAndKeyFileName)
			cert := createTlsCert(t, domain)
			err := certToDisk(cert, certPath)
			attest.Ok(t, err)
		}

		m, err := initManager([]string{domain}, email, acmeDirectoryUrl, l, testDiskCache)
		attest.Ok(t, err)
		attest.NotZero(t, m)

		cert := m.getCertFastPath(domain)
		attest.NotZero(t, cert)
		attest.True(t, certIsValid(cert))
	})
}

func TestGetCertificate(t *testing.T) {
	t.Parallel()

	email := "hey+sample@gmail.com"
	acmeDirectoryUrl := "https://some-domain.com/directory"

	l := slog.Default()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		domain := getDomain()
		diskCacheDir, errA := diskCachedir()
		attest.Ok(t, errA)
		{ // prep by saving a certificate for the domain to disk.
			certPath := filepath.Join(diskCacheDir, domain, certAndKeyFileName)
			cert := createTlsCert(t, domain)
			errB := certToDisk(cert, certPath)
			attest.Ok(t, errB)
		}

		getCrt := GetCertificate([]string{domain}, email, acmeDirectoryUrl, l)
		cert, errC := getCrt(&tls.ClientHelloInfo{
			ServerName: domain,
		})
		attest.Ok(t, errC)
		attest.NotZero(t, cert)
		attest.True(t, certIsValid(cert))
	})

	t.Run("domain is IP address", func(t *testing.T) {
		t.Parallel()

		domain := "127.0.0.1"
		getCrt := GetCertificate([]string{domain}, email, acmeDirectoryUrl, l)
		cert, errC := getCrt(&tls.ClientHelloInfo{
			ServerName: domain,
		})
		attest.Zero(t, cert)
		attest.False(t, certIsValid(cert))
		attest.Error(t, errC)
	})
}

func someAcmeAppHandler(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, msg)
	}
}

func TestAcmeHandler(t *testing.T) {
	t.Parallel()

	l := slog.Default()
	email := "hey+sample@gmail.com"

	t.Run("normal request succeeds", func(t *testing.T) {
		t.Parallel()

		msg := "hello world"
		wrappedHandler := Handler(someAcmeAppHandler(msg))
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Subsequence(t, string(rb), msg)
	})

	t.Run("cert request from acme succeeds", func(t *testing.T) {
		t.Parallel()

		domain := getDomain()
		ts := httptest.NewServer(someAcmeServerHandler(t, domain))
		t.Cleanup(func() {
			ts.Close()
		})

		acmeDirectoryUrl, errA := url.JoinPath(ts.URL, "/directory")
		attest.Ok(t, errA)

		{ // initialize manager.
			getCrt := GetCertificate([]string{domain}, email, acmeDirectoryUrl, l)
			attest.NotZero(t, getCrt)
			cert, errB := getCrt(&tls.ClientHelloInfo{
				ServerName: domain,
			})
			attest.Ok(t, errB)
			attest.NotZero(t, cert)
			attest.True(t, certIsValid(cert))
		}

		token := ""
		{
			diskCacheDir, err := diskCachedir()
			attest.Ok(t, err)
			tokenPath := filepath.Join(diskCacheDir, domain, tokenFileName)
			tok, err := os.ReadFile(tokenPath)
			attest.Ok(t, err)
			token = string(tok)
		}

		msg := "hello"
		wrappedHandler := Handler(someAcmeAppHandler(msg))
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("%s%s", challengeURI, token), nil)
		req.Host = domain
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, errC := io.ReadAll(res.Body)
		attest.Ok(t, errC)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Subsequence(
			t,
			string(rb),
			// This message is from `someAcmeServerHandler`
			"token",
		)
	})

	t.Run("cert request from memory succeeds", func(t *testing.T) {
		t.Parallel()

		domain := getDomain()
		ts := httptest.NewServer(someAcmeServerHandler(t, domain))
		t.Cleanup(func() {
			ts.Close()
		})

		acmeDirectoryUrl, errA := url.JoinPath(ts.URL, "/directory")
		attest.Ok(t, errA)
		testDiskCache := t.TempDir()

		m, err := initManager([]string{domain}, email, acmeDirectoryUrl, l, testDiskCache)
		attest.Ok(t, err)
		attest.NotZero(t, m)

		{ // Flush the cache.
			m.cache = newCache()
		}

		{ // Get cert will fetch from ACME and fill memory/cache.
			cert, errB := m.getCert(context.Background(), domain)
			attest.Ok(t, errB)
			attest.NotZero(t, cert)
		}

		{ // Get cert should now fetch from memory.
			errC := os.RemoveAll(testDiskCache)
			attest.Ok(t, errC)
			m.acmeDirectoryUrl = ""
			attest.Zero(t, m.acmeDirectoryUrl)

			cert, errD := m.getCert(context.Background(), domain)
			attest.Ok(t, errD)
			attest.NotZero(t, cert)
		}
	})

	t.Run("request error cases", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := Handler(someAcmeAppHandler(msg))
		token := "myToken"

		tests := []struct {
			name string
			req  func() *http.Request
			code int
			resp string
		}{
			{
				name: "normal request",
				req: func() *http.Request {
					domain := getDomain()
					{
						diskCacheDir, err := diskCachedir()
						attest.Ok(t, err)

						certPath := filepath.Join(diskCacheDir, domain, tokenFileName)
						err = os.MkdirAll(filepath.Join(diskCacheDir, domain), 0o755)
						attest.Ok(t, err)

						err = os.WriteFile(certPath, []byte(token), 0o600)
						attest.Ok(t, err)
					}

					r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("%s%s", challengeURI, token), nil)
					r.Host = domain
					return r
				},
				code: http.StatusOK,
				resp: token,
			},
			{
				name: "https request",
				req: func() *http.Request {
					domain := getDomain()
					r := httptest.NewRequest(http.MethodGet, challengeURI, nil)
					r.Host = domain
					r.TLS = &tls.ConnectionState{}
					return r
				},
				code: http.StatusTeapot,
				resp: "request should not be https",
			},
			{
				name: "ip address",
				req: func() *http.Request {
					r := httptest.NewRequest(http.MethodGet, challengeURI, nil)
					r.Host = "127.0.0.1"
					return r
				},
				code: http.StatusTeapot,
				resp: "should not be IP address",
			},
			{
				// see; https://github.com/komuw/ong/issues/327
				name: "subdomain with number",
				req: func() *http.Request {
					domain := "2023.example.com"
					{
						diskCacheDir, err := diskCachedir()
						attest.Ok(t, err)

						certPath := filepath.Join(diskCacheDir, domain, tokenFileName)
						err = os.MkdirAll(filepath.Join(diskCacheDir, domain), 0o755)
						attest.Ok(t, err)

						err = os.WriteFile(certPath, []byte(token), 0o600)
						attest.Ok(t, err)
					}

					r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("%s%s", challengeURI, token), nil)
					r.Host = "2023.example.com"
					return r
				},
				code: http.StatusOK,
				resp: token,
			},
			{
				name: "domain with port",
				req: func() *http.Request {
					domain := "example.com"
					{
						diskCacheDir, err := diskCachedir()
						attest.Ok(t, err)

						certPath := filepath.Join(diskCacheDir, domain, tokenFileName)
						err = os.MkdirAll(filepath.Join(diskCacheDir, domain), 0o755)
						attest.Ok(t, err)

						err = os.WriteFile(certPath, []byte(token), 0o600)
						attest.Ok(t, err)
					}

					r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("%s%s", challengeURI, token), nil)
					r.Host = fmt.Sprintf("%s:2023", domain)
					return r
				},
				code: http.StatusOK,
				resp: token,
			},
			{
				name: "no token found",
				req: func() *http.Request {
					domain := getDomain()
					r := httptest.NewRequest(http.MethodGet, challengeURI, nil)
					r.Host = domain
					return r
				},
				code: http.StatusInternalServerError,
				resp: "no such file or directory",
			},
		}

		for _, tt := range tests {
			tt := tt

			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				rec := httptest.NewRecorder()
				wrappedHandler.ServeHTTP(rec, tt.req())

				res := rec.Result()
				defer res.Body.Close()

				rb, err := io.ReadAll(res.Body)
				attest.Ok(t, err)

				attest.Equal(t, res.StatusCode, tt.code)
				attest.Subsequence(t, string(rb), tt.resp)
			})
		}
	})
}

func BenchmarkGetCertificate(b *testing.B) {
	b.Run("success", func(b *testing.B) {
		email := "hey+sample@gmail.com"
		acmeDirectoryUrl := "https://some-domain.com/directory"
		l := slog.Default()

		domain := getDomain()
		diskCacheDir, errA := diskCachedir()
		attest.Ok(b, errA)
		{ // prep by saving a certificate for the domain to disk.
			certPath := filepath.Join(diskCacheDir, domain, certAndKeyFileName)
			cert := createTlsCert(b, domain)
			errB := certToDisk(cert, certPath)
			attest.Ok(b, errB)
		}
		getCrt := GetCertificate([]string{domain}, email, acmeDirectoryUrl, l)

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			cert, errC := getCrt(&tls.ClientHelloInfo{
				ServerName: domain,
			})
			attest.Ok(b, errC)
			attest.NotZero(b, cert)
			attest.True(b, certIsValid(cert))
		}
	})
}

func BenchmarkHandler(b *testing.B) {
	domain := getDomain()
	msg := "hello"
	wrappedHandler := Handler(someAcmeAppHandler(msg))

	{
		diskCacheDir, err := diskCachedir()
		attest.Ok(b, err)

		token := "myToken"
		certPath := filepath.Join(diskCacheDir, domain, tokenFileName)
		err = os.MkdirAll(filepath.Join(diskCacheDir, domain), 0o755)
		attest.Ok(b, err)

		err = os.WriteFile(certPath, []byte(token), 0o600)
		attest.Ok(b, err)
	}

	b.Run("success", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, challengeURI, nil)
			req.Host = domain
			wrappedHandler.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			attest.Equal(b, res.StatusCode, http.StatusOK)
		}
	})
}
