package acme

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/idna"
)

// diskCachedir is directory for acme configs like certificates.
func diskCachedir() (string, error) {
	/* The file hierarchy document[1] says that `/dev/shm` is world-writable and should be avoided
	They recommend memor-mapped files in $XDG_RUNTIME_DIR or `/run`
	Other password managers[2][3][4] however do use `/dev/shm`.
	Rather than re-invent the wheel, we'll use that.

	1. https://www.freedesktop.org/software/systemd/man/file-hierarchy.html
	2. https://github.com/FiloSottile/passage/blob/1.7.4a1/src/password-store.sh#L175-L191
	3. https://git.zx2c4.com/password-store/tree/src/password-store.sh#n216
	4. https://github.com/gopasspw/gopass/blob/v1.15.3-rc1/pkg/tempfile/mount_linux.go#L13-L27
	*/

	dir, _ := os.UserConfigDir()
	if dir == "" {
		dir = "/dev/shm"
	}
	if dir == "" {
		dir = "/tmp/"
	}
	if testing.Testing() {
		// Set dir==/tmp/ for tests, so that we do not fill the disk.
		dir = "/tmp/"
	}

	dir = filepath.Join(dir, "ong_acme")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		// If path is already a directory, MkdirAll does nothing
		return "", fmt.Errorf("ong/acme: unable to create directory %s: %w", dir, err)
	}
	return dir, nil
}

// jwkEncode encodes public part of an ECDSA key into a JWK.
// The result is also suitable for creating a JWK thumbprint.
// https://tools.ietf.org/html/rfc7517
//
// see: https://github.com/golang/crypto/blob/v0.10.0/acme/jws.go#L157-L160
func jwkEncode(pub ecdsa.PublicKey) jwk {
	// https://tools.ietf.org/html/rfc7518#section-6.2.1
	p := pub.Curve.Params()
	n := p.BitSize / 8
	if p.BitSize%8 != 0 {
		n++
	}
	x := pub.X.Bytes()
	if n > len(x) {
		x = append(make([]byte, n-len(x)), x...)
	}
	y := pub.Y.Bytes()
	if n > len(y) {
		y = append(make([]byte, n-len(y)), y...)
	}

	return jwk{
		// Field order is important.
		// See https://tools.ietf.org/html/rfc7638#section-3.3 for details.
		Crv: p.Name,
		Kty: "EC",
		X:   base64.RawURLEncoding.EncodeToString(x),
		Y:   base64.RawURLEncoding.EncodeToString(y),
	}
}

// jwsHasher indicates suitable JWS algorithm name and a hash function
// to use for signing a digest with the provided key.
// It returns ("", 0) if the key is not supported.
func jwsHasher(pub ecdsa.PublicKey) (string, crypto.Hash) {
	switch pub.Params().Name {
	case "P-256":
		return "ES256", crypto.SHA256
	case "P-384":
		return "ES384", crypto.SHA384
	case "P-521":
		return "ES512", crypto.SHA512
	default:
		// This package only deals with `ecdsa.PublicKey`.
		// So, if we get here, it must be a programmer error.
		panic(fmt.Sprintf("ong/acme: unknown ecdsa.PublicKey param; %v", pub.Params().Name))
	}
}

// jwsSign signs the digest using the given key.
// The hash is unused for ECDSA keys.
func jwsSign(key *ecdsa.PrivateKey, hash crypto.Hash, digest []byte) ([]byte, error) {
	sigASN1, errA := key.Sign(rand.Reader, digest, hash)
	if errA != nil {
		return nil, errA
	}

	var rs struct{ R, S *big.Int }
	if _, errB := asn1.Unmarshal(sigASN1, &rs); errB != nil {
		return nil, errB
	}

	rb, sb := rs.R.Bytes(), rs.S.Bytes()
	size := key.PublicKey.Params().BitSize / 8
	if size%8 > 0 {
		size++
	}
	sig := make([]byte, size*2)
	copy(sig[size-len(rb):], rb)
	copy(sig[size*2-len(sb):], sb)

	return sig, nil
}

// encodeECDSAKey writes privateKey to w.
func encodeECDSAKey(w io.Writer, key *ecdsa.PrivateKey) error {
	b, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	pb := &pem.Block{Type: "EC PRIVATE KEY", Bytes: b}
	return pem.Encode(w, pb)
}

// Attempt to parse the given private key DER block. OpenSSL 0.9.8 generates
// PKCS#1 private keys by default, while OpenSSL 1.0.0 generates PKCS#8 keys.
// OpenSSL ecparam generates SEC1 EC private keys for ECDSA. We try all three.
//
// Inspired by parsePrivateKey in crypto/tls/tls.go.
func parsePrivateKey(der []byte) (*ecdsa.PrivateKey, error) {
	// https://github.com/golang/crypto/blob/v0.10.0/acme/autocert/autocert.go#L1078

	key, err := x509.ParseECPrivateKey(der)
	if err != nil {
		return nil, err
	}

	// If this ever becomes a problem, we should also include `x509.ParsePKCS8PrivateKey()`
	// See the code in link; `autocert/autocert.go`

	return key, nil
}

// jWKThumbprint creates a JWK thumbprint out of pub.
// https://tools.ietf.org/html/rfc7638.
//
// When ACME calls your server at `http://myDomain.com/.well-known/acme-challenge/<token>`
// Your server should respond with the value returned by this function.
func jWKThumbprint(pub ecdsa.PublicKey, token string) (string, error) {
	jwk := jwkEncode(pub)
	jwkBytes, err := json.Marshal(jwk)
	if err != nil {
		return "", err
	}

	b := sha256.Sum256(jwkBytes)
	thumb := base64.RawURLEncoding.EncodeToString(b[:])
	return fmt.Sprintf("%s.%s", token, thumb), nil
}

func getLeaf(der [][]byte) (leaf *x509.Certificate, err error) {
	// https://github.com/golang/crypto/blob/v0.10.0/acme/autocert/autocert.go#L1105-L1110

	// parse public part(s)
	var n int
	for _, b := range der {
		n += len(b)
	}
	pub := make([]byte, n)
	n = 0
	for _, b := range der {
		n += copy(pub[n:], b)
	}
	x509Cert, err := x509.ParseCertificates(pub)
	if err != nil || len(x509Cert) == 0 {
		return nil, errors.New("acme/autocert: no public key found")
	}
	leaf = x509Cert[0]

	return leaf, nil
}

func certFromDisk(certPath string) (*tls.Certificate, error) {
	if err := os.MkdirAll(filepath.Dir(certPath), 0o755); err != nil {
		// If directory already exists, MkdirAll does nothing.
		return nil, err
	}

	f, err := os.Open(certPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	return certFromReader(f)
}

func certFromReader(r io.Reader) (*tls.Certificate, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxCertChainSize))
	if err != nil {
		return nil, err
	}

	// private
	priv, pub := pem.Decode(data)
	if priv == nil || !strings.Contains(priv.Type, "PRIVATE") {
		return nil, errors.New("ong/acme: bad cert private key")
	}
	privKey, err := parsePrivateKey(priv.Bytes)
	if err != nil {
		return nil, err
	}

	// public
	var pubDER [][]byte
	for len(pub) > 0 {
		var b *pem.Block
		b, pub = pem.Decode(pub)
		if b == nil {
			break
		}
		pubDER = append(pubDER, b.Bytes)
	}
	if len(pub) > 0 {
		// Leftover content not consumed by pem.Decode. Corrupt. Ignore.
		return nil, errors.New("ong/acme: bad cert public key")
	}

	leaf, err := getLeaf(pubDER)
	if err != nil {
		return nil, err
	}

	cert := &tls.Certificate{
		Certificate: pubDER,
		PrivateKey:  privKey,
		Leaf:        leaf,
	}

	if !certIsValid(cert) {
		return nil, errors.New("ong/acme: certificate is expired")
	}

	return cert, nil
}

func certToDisk(cert *tls.Certificate, certPath string) error {
	if err := os.MkdirAll(filepath.Dir(certPath), 0o755); err != nil {
		// If directory already exists, MkdirAll does nothing.
		return err
	}

	// contains PEM-encoded data
	var buf bytes.Buffer

	// private
	switch key := cert.PrivateKey.(type) {
	case *ecdsa.PrivateKey:
		if err := encodeECDSAKey(&buf, key); err != nil {
			return err
		}
	default:
		// This package only deals with `ecdsa.PublicKey`.
		// So, if we get here, it must be a programmer error.
		return fmt.Errorf("ong/acme: unknown type of PrivateKey; %v", key)
	}

	// public
	for _, b := range cert.Certificate {
		pb := &pem.Block{Type: "CERTIFICATE", Bytes: b}
		if err := pem.Encode(&buf, pb); err != nil {
			return err
		}
	}

	return os.WriteFile(certPath, buf.Bytes(), 0o600)
}

// certIsValid reports whether a certificate is valid.
func certIsValid(cert *tls.Certificate) bool {
	// check validity
	// todo: add more validation checks,
	// see: https://github.com/golang/crypto/blob/v0.10.0/acme/autocert/autocert.go#L1105-L1110
	if cert == nil {
		return false
	}

	// Let's encrypt backdates certificates by one hour to allow for clock skew.
	// See: https://community.letsencrypt.org/t/time-zone-considerations-needed-for-certificates/23130/2
	now := time.Now().UTC()
	threeDaysAfter := now.Add(3 * 24 * time.Hour)

	if now.Before(cert.Leaf.NotBefore) {
		// certificate is too early.
		return false
	}
	if now.After(cert.Leaf.NotAfter) {
		// certificate is expired.
		return false
	}

	if threeDaysAfter.After(cert.Leaf.NotAfter) {
		// certificate is almost expired.
		return false
	}

	return true
}

// customHostWhitelist is modeled after `autocert.HostWhitelist` except that it allows wildcards.
// However, the certificate issued will NOT be wildcard certs; since letsencrypt only issues wildcard certs via DNS-01 challenge
// Instead, we'll get a certificate per subdomain.
// see; https://letsencrypt.org/docs/faq/#does-let-s-encrypt-issue-wildcard-certificates
//
// HostWhitelist returns a policy where only the specified domain names are allowed.
//
// Note that all domain will be converted to Punycode via idna.Lookup.ToASCII so that
// Manager.GetCertificate can handle the Unicode IDN and mixedcase domain correctly.
// Invalid domain will be silently ignored.
func customHostWhitelist(domain string) hostPolicy {
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

	return func(host string) error {
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

// retryAfter parses a Retry-After HTTP header value,
// trying to convert v into an int (seconds) or use http.ParseTime otherwise.
// It returns zero value if v cannot be parsed.
func retryAfter(v string, fallback time.Duration) time.Duration {
	if i, err := strconv.Atoi(v); err == nil {
		return time.Duration(i) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		return t.Sub(time.Now().UTC())
	}
	return fallback
}
