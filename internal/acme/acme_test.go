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
	"math/big"
	mathRand "math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.akshayshah.org/attest"
	"golang.org/x/exp/slog"
)

// getDomain returns a valid unique domain.
func getDomain() string {
	r := mathRand.Intn(100_000) + 1
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

		getCrt := GetCertificate(domain, email, acmeDirectoryUrl, l)
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
		m := initManager(domain, email, acmeDirectoryUrl, l, testDiskCache)
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

		m := initManager(domain, email, acmeDirectoryUrl, l, testDiskCache)
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

		m := initManager(domain, email, acmeDirectoryUrl, l, testDiskCache)
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

		m := initManager(domain, email, acmeDirUrl, l, testDiskCache)
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

		m := initManager(domain, email, acmeDirectoryUrl, l, testDiskCache)
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

		getCrt := GetCertificate(domain, email, acmeDirectoryUrl, l)
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
		getCrt := GetCertificate(domain, email, acmeDirectoryUrl, l)
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

		msg := "hello"
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
			getCrt := GetCertificate(domain, email, acmeDirectoryUrl, l)
			attest.NotZero(t, getCrt)
			cert, errB := getCrt(&tls.ClientHelloInfo{
				ServerName: domain,
			})
			attest.Ok(t, errB)
			attest.NotZero(t, cert)
			attest.True(t, certIsValid(cert))
		}

		msg := "hello"
		wrappedHandler := Handler(someAcmeAppHandler(msg))
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, challengeURI, nil)
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

		m := initManager(domain, email, acmeDirectoryUrl, l, testDiskCache)
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
		getCrt := GetCertificate(domain, email, acmeDirectoryUrl, l)

		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			cert, errC := getCrt(&tls.ClientHelloInfo{
				ServerName: domain,
			})
			attest.Ok(b, errC)
			attest.NotZero(b, cert)
			attest.True(b, certIsValid(cert))
		}
	})
}
