package server

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

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

// customHostWhitelist is modeled after `autocert.HostWhitelist``
//
// HostWhitelist returns a policy where only the specified domain names are allowed.
// Only exact matches are currently supported. Subdomains, regexp or wildcard
// will not match.
//
// Note that all domain will be converted to Punycode via idna.Lookup.ToASCII so that
// Manager.GetCertificate can handle the Unicode IDN and mixedcase domain correctly.
// Invalid domain will be silently ignored.
func customHostWhitelist(domain string) autocert.HostPolicy {
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
			exactMatch = strings.ReplaceAll(domain, "*", "")
			exactMatch = strings.TrimLeft(exactMatch, ".")
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

// getTlsConfig returns a proper tls configuration given the options passed in.
// The tls config may either procure certifiates from LetsEncrypt, from disk or be nil(for non-tls traffic)
func getTlsConfig(o opts, logger log.Logger) (*tls.Config, error) {
	if o.tls.email != "" {
		// 1. use letsencrypt.
		//

		if strings.Count(o.tls.domain, "*") > 1 {
			return nil, ongErrors.New("domain can only contain one wildcard character")
		}
		if strings.Contains(o.tls.domain, "*") && !strings.HasPrefix(o.tls.domain, "*") {
			return nil, ongErrors.New("wildcard character should be a prefix")
		}

		const letsEncryptProductionUrl = "https://acme-v02.api.letsencrypt.org/directory"
		const letsEncryptStagingUrl = "https://acme-staging-v02.api.letsencrypt.org/directory"
		m := &autocert.Manager{
			Client: &acme.Client{DirectoryURL: letsEncryptStagingUrl},
			Cache:  autocert.DirCache("ong-certifiate-dir"),
			Prompt: autocert.AcceptTOS,
			Email:  o.tls.email,
			HostPolicy: autocert.HostWhitelist(
				// todo: replace this with our own function.
				// note: the func(`autocert.HostWhitelist`) does only exact matches. Subdomains, regexp or wildcard will not match.
				//       we should change that.
				"example.org",
				"www.example.org",
			),
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

var certLogger = log.New( //nolint:gochecknoglobals
	context.Background(),
	os.Stdout,
	100).
	WithImmediate().
	WithFields(
		log.F{"pid": os.Getpid()},
	)

// CreateDevCertKey generates and saves(to disk) a certifiate and key that can be used to configure a tls server.
// This is only meant to be used for development/local settings.
// The certificate is self-signed & a best effort is made to add its CA to the OS trust store.
func CreateDevCertKey() (certFile, keyFile string) {
	certLogger.Info(log.F{"msg": "creating dev tls cert and key"})
	defer certLogger.Info(log.F{"msg": "done creating dev tls cert and key"})

	caCert, caKey := installCA()
	certPath, keyPath := certKeyPaths()

	privKey := generatePrivKey()
	pubKey := privKey.(crypto.Signer).Public()

	certTemplate := &x509.Certificate{
		SerialNumber: randomSerialNumber(),
		Subject: pkix.Name{
			Organization:       []string{"ong development certificate"},
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
	certLogger.Info(log.F{"msg": "installing dev root CA"})
	defer certLogger.Info(log.F{"msg": "done installing dev root CA"})

	caCert, caKey = loadCA()

	caUniqename := "ong_development_CA"
	systemTrustFilename := func() string {
		// https://ubuntu.com/server/docs/security-trust-store
		sysTrustFname := "/usr/local/share/ca-certificates/%s.crt"
		return fmt.Sprintf(sysTrustFname, caUniqename)
	}

	_, errStat := os.Stat(systemTrustFilename())
	_, errVerify := caCert.Verify(x509.VerifyOptions{})
	if errVerify == nil && errStat == nil {
		// cert is already installed.
		certLogger.Info(log.F{"msg": "root CA was already installed"})
		return caCert, caKey
	}

	rootCACertName, _ := rootCAcertKeyPaths()
	cert, err := os.ReadFile(rootCACertName)
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
	certLogger.Info(log.F{"msg": string(out)})
	if err != nil {
		panic(err)
	}

	installInNss := func() {
		// certutil -V -d ~/.pki/nssdb -u L -n caUniqename # validate cert in NSS store.

		u, errC := user.Current()
		if errC != nil {
			panic(errC)
		}
		nssDb := filepath.Join(u.HomeDir, ".pki/nssdb")

		delete := []string{"certutil", "-D", "-d", nssDb, "-n", caUniqename}
		cmd = commandWithSudo(delete...)
		out, err = cmd.CombinedOutput()
		_ = err // ignore error

		add := []string{"certutil", "-A", "-d", nssDb, "-t", "C,,", "-n", caUniqename, "-i", rootCACertName}
		cmd = commandWithSudo(add...)
		out, err = cmd.CombinedOutput()
		certLogger.Info(log.F{"msg": string(out), "args": cmd.Args})
		if err != nil {
			panic(err)
		}
	}
	installInNss()

	return caCert, caKey
}

func loadCA() (caCert *x509.Certificate, caKey any) {
	rootCACertName, rootCAKeyName := rootCAcertKeyPaths()
	if _, err := os.Stat(rootCACertName); err != nil {
		newCA()
	}

	certPEMBlock, err := os.ReadFile(rootCACertName)
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

	keyPEMBlock, err := os.ReadFile(rootCAKeyName)
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
			Organization:       []string{"ong development CA"},
			OrganizationalUnit: []string{getOrg()},
			// The CommonName is required by iOS to show the certificate in the
			// "Certificate Trust Settings" menu.
			// https://github.com/FiloSottile/mkcert/issues/47
			CommonName: "ong " + getOrg(),
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

	err = os.WriteFile(
		rootCACertName,
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert}),
		0o666,
	)
	if err != nil {
		panic(err)
	}

	err = os.WriteFile(
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

func rootCAcertKeyPaths() (string, string) {
	const rootCACertName = "rootCA_cert.pem"
	const rootCAKeyName = "rootCA_key.pem"

	getCArootpath := func() string {
		u, err := user.Current()
		if err != nil {
			return "/tmp/ong"
		}
		return filepath.Join(u.HomeDir, "ong")
	}
	caRoot := getCArootpath()
	if _, err := os.Stat(caRoot); err != nil {
		errMk := os.MkdirAll(caRoot, 0o761)
		if errMk != nil {
			panic(errMk)
		}
	}

	return filepath.Join(caRoot, rootCACertName), filepath.Join(caRoot, rootCAKeyName)
}

func certKeyPaths() (string, string) {
	const certPath = "/tmp/ong_dev_certificate.pem"
	const keyPath = "/tmp/ong_dev_key.pem"
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
		name = "ong-org"
	}
	return name
}
