package server

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"time"
)

// Most of the code here is insipired(or taken from) by:
//   (a) https://github.com/eliben/code-for-blog whose license(Unlicense) can be found here:     https://github.com/eliben/code-for-blog/blob/464a32f686d7646ba3fc612c19dbb550ec8a05b1/LICENSE
//   (b) https://github.com/FiloSottile/mkcert   whose license(BSD 3-Clause ) can be found here: https://github.com/FiloSottile/mkcert/blob/v1.4.4/LICENSE

// CreateDevCertKey generates and saves(to disk) a certifiate and key that can be used to configure a tls server.
// This is only meant to be used for development/local settings. The certificate is self-signed.
func CreateDevCertKey() (certFile, keyFile string) {
	const certPath = "/tmp/goweb_dev_certificate.pem"
	const keyPath = "/tmp/goweb_dev_key.pem"

	generatePrivKey := func() (key crypto.PrivateKey) {
		var err error
		if key, err = rsa.GenerateKey(rand.Reader, 2048); err != nil {
			// fallback
			key, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		}
		if err != nil {
			panic(err)
		}
		return key
	}

	randomSerialNumber := func() *big.Int {
		serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
		serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
		if err != nil {
			panic(err)
		}
		return serialNumber
	}

	org := func() string {
		name, err := os.Hostname()
		if err == nil {
			name = "goweb-org"
		}
		return name
	}

	privKey := generatePrivKey()
	pubKey := privKey.(crypto.Signer).Public()

	certTemplate := &x509.Certificate{
		SerialNumber: randomSerialNumber(),
		Subject: pkix.Name{
			Organization:       []string{"goweb development certificate"},
			OrganizationalUnit: []string{org()},
		},
		DNSNames:  []string{"localhost"},
		NotBefore: time.Now(),
		// The maximum for `NotAfter` should be 825days
		// See https://support.apple.com/en-us/HT210176
		NotAfter:    time.Now().Add(1 * time.Hour),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}

	cert, err := x509.CreateCertificate(rand.Reader, certTemplate, certTemplate, pubKey, privKey)
	if err != nil {
		panic(err)
	}

	pemCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert})
	if err := os.WriteFile(certPath, pemCert, 0o644); err != nil {
		panic(err)
	}

	key, err := x509.MarshalPKCS8PrivateKey(privKey)
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key})
	if err := os.WriteFile(keyPath, pemKey, 0o600); err != nil {
		panic(err)
	}

	return certPath, keyPath
}
