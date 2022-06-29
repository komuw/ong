package server

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"time"

	"github.com/komuw/goweb/id"
	"github.com/komuw/goweb/log"
)

// Most of the code here is insipired(or taken from) by:
//   (a) https://github.com/eliben/code-for-blog whose license(Unlicense) can be found here:     https://github.com/eliben/code-for-blog/blob/464a32f686d7646ba3fc612c19dbb550ec8a05b1/LICENSE
//   (b) https://github.com/FiloSottile/mkcert   whose license(BSD 3-Clause ) can be found here: https://github.com/FiloSottile/mkcert/blob/v1.4.4/LICENSE

// TODO: unexport
var logger = log.New(context.Background(), os.Stdout, 1000, false).WithImmediate().WithFields(log.F{"pid": os.Getpid()})

// CreateDevCertKey generates and saves(to disk) a certifiate and key that can be used to configure a tls server.
// This is only meant to be used for development/local settings. The certificate is self-signed.
func CreateDevCertKey() (certFile, keyFile string) {
	logger.Info(log.F{"msg": "creating dev tls cert and key"})
	defer logger.Info(log.F{"msg": "done creating dev tls cert and key"})

	caCert, caKey := installCA()
	certPath, keyPath := certKeyPaths()

	privKey := generatePrivKey()
	pubKey := privKey.(crypto.Signer).Public()

	certTemplate := &x509.Certificate{
		SerialNumber: randomSerialNumber(),
		Subject: pkix.Name{
			Organization:       []string{"goweb development certificate"},
			OrganizationalUnit: []string{getOrg()},
		},
		DNSNames:  []string{"localhost"},
		NotBefore: time.Now(),
		// The maximum for `NotAfter` should be 825days
		// See https://support.apple.com/en-us/HT210176
		NotAfter:    time.Now().Add(8 * time.Hour),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}

	cert, err := x509.CreateCertificate(rand.Reader, certTemplate, caCert, pubKey, caKey)
	if err != nil {
		panic(err)
	}

	pemCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert})
	if err = os.WriteFile(certPath, pemCert, 0o644); err != nil {
		panic(err)
	}

	key, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		panic(err)
	}
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key})
	if err = os.WriteFile(keyPath, pemKey, 0o600); err != nil {
		panic(err)
	}

	return certPath, keyPath
}

// installCA adds the dev root CA to the linux/ubuntu certificate trust store.
func installCA() (caCert *x509.Certificate, caKey any) {
	logger.Info(log.F{"msg": "installing dev root CA"})
	defer logger.Info(log.F{"msg": "done installing dev root CA"})

	caCert, caKey = loadCA()

	_, err := caCert.Verify(x509.VerifyOptions{})
	if err == nil {
		// cert is already installed.
		logger.Info(log.F{"msg": "root CA was already installed"})
		return caCert, caKey
	}

	rootCACertName, _ := rootCAcertKeyPaths()
	cert, err := ioutil.ReadFile(rootCACertName)
	if err != nil {
		panic(err)
	}

	cmd := commandWithSudo("tee", systemTrustFilename())
	cmd.Stdin = bytes.NewReader(cert)
	out, err := cmd.CombinedOutput()
	if err != nil {
		panic(err)
	}

	sysTrustCmd := []string{"update-ca-certificates"}
	cmd = commandWithSudo(sysTrustCmd...)
	out, err = cmd.CombinedOutput()
	logger.Info(log.F{"msg": string(out)})
	if err != nil {
		panic(err)
	}

	return caCert, caKey
}

func loadCA() (caCert *x509.Certificate, caKey any) {
	rootCACertName, rootCAKeyName := rootCAcertKeyPaths()
	if _, err := os.Stat(rootCACertName); err != nil {
		newCA()
	}

	certPEMBlock, err := ioutil.ReadFile(rootCACertName)
	if err != nil {
		panic(err)
	}

	certDERBlock, _ := pem.Decode(certPEMBlock)
	if certDERBlock == nil || certDERBlock.Type != "CERTIFICATE" {
		panic("failed to read CA cert.")
	}

	caCert, err = x509.ParseCertificate(certDERBlock.Bytes)
	if err != nil {
		panic(err)
	}

	keyPEMBlock, err := ioutil.ReadFile(rootCAKeyName)
	if err != nil {
		panic(err)
	}

	keyDERBlock, _ := pem.Decode(keyPEMBlock)
	if keyDERBlock == nil || keyDERBlock.Type != "PRIVATE KEY" {
		panic("failed to read CA key.")
	}
	caKey, err = x509.ParsePKCS8PrivateKey(keyDERBlock.Bytes)
	if err != nil {
		panic(err)
	}

	return caCert, caKey
}

func newCA() {
	rootCACertName, rootCAKeyName := rootCAcertKeyPaths()

	privKey := generatePrivKey()
	pubKey := privKey.(crypto.Signer).Public()

	spkiASN1, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		panic(err)
	}

	var spki struct {
		Algorithm        pkix.AlgorithmIdentifier
		SubjectPublicKey asn1.BitString
	}
	_, err = asn1.Unmarshal(spkiASN1, &spki)
	if err != nil {
		panic(err)
	}

	skid := sha1.Sum(spki.SubjectPublicKey.Bytes)
	tpl := &x509.Certificate{
		SerialNumber: randomSerialNumber(),
		Subject: pkix.Name{
			Organization:       []string{"goweb development CA"},
			OrganizationalUnit: []string{getOrg()},
			// The CommonName is required by iOS to show the certificate in the
			// "Certificate Trust Settings" menu.
			// https://github.com/FiloSottile/mkcert/issues/47
			CommonName: "goweb " + getOrg(),
		},
		SubjectKeyId:          skid[:],
		NotAfter:              time.Now().AddDate(10, 0, 0), // 10years
		NotBefore:             time.Now(),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}

	cert, err := x509.CreateCertificate(rand.Reader, tpl, tpl, pubKey, privKey)
	if err != nil {
		panic(err)
	}

	privDER, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		panic(err)
	}

	err = ioutil.WriteFile(
		rootCACertName,
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert}),
		0o666,
	)
	if err != nil {
		panic(err)
	}

	err = ioutil.WriteFile(
		rootCAKeyName,
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER}),
		0o400,
	)
	if err != nil {
		panic(err)
	}
}

func commandWithSudo(cmd ...string) *exec.Cmd {
	if u, err := user.Current(); err == nil && u.Uid == "0" {
		return exec.Command(cmd[0], cmd[1:]...)
	}
	return exec.Command("sudo", append([]string{"--prompt=Sudo password:", "--"}, cmd...)...)
}

func systemTrustFilename() string {
	uniqename := "goweb_development_CA_" + id.New()
	sysTrustFname := "/usr/local/share/ca-certificates/%s.crt"
	return fmt.Sprintf(sysTrustFname, uniqename)
}

func rootCAcertKeyPaths() (string, string) {
	const rootCACertName = "rootCA_cert.pem"
	const rootCAKeyName = "rootCA_key.pem"

	getCArootpath := func() string {
		u, err := user.Current()
		if err != nil {
			return "/tmp/goweb"
		}
		return filepath.Join(u.HomeDir, "goweb")
	}
	caRoot := getCArootpath()
	if _, err := os.Stat(caRoot); err != nil {
		err := os.MkdirAll(caRoot, 0o761)
		if err != nil {
			panic(err)
		}
	}

	return filepath.Join(caRoot, rootCACertName), filepath.Join(caRoot, rootCAKeyName)
}

func certKeyPaths() (string, string) {
	const certPath = "/tmp/goweb_dev_certificate.pem"
	const keyPath = "/tmp/goweb_dev_key.pem"
	return certPath, keyPath
}

func generatePrivKey() (key crypto.PrivateKey) {
	var err error
	key, err = rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		panic(err)
	}
	return key
}

func randomSerialNumber() *big.Int {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		panic(err)
	}
	return serialNumber
}

func getOrg() string {
	name, err := os.Hostname()
	if err == nil {
		name = "goweb-org"
	}
	return name
}
