package config

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"time"

	"github.com/komuw/ong/errors"
)

// Some of the code here is inspired(or taken from) by:
//   (a) https://github.com/eliben/code-for-blog whose license(Unlicense) can be found here:                                   https://github.com/eliben/code-for-blog/blob/464a32f686d7646ba3fc612c19dbb550ec8a05b1/LICENSE
//   (b) https://github.com/FiloSottile/mkcert   whose license(BSD 3-Clause ) can be found here:                               https://github.com/FiloSottile/mkcert/blob/v1.4.4/LICENSE
//   (c) https://github.com/golang/crypto/blob/master/acme/autocert/autocert.go whose license(BSD 3-Clause) can be found here: https://github.com/golang/crypto/blob/v0.18.0/LICENSE
//   (d) https://github.com/caddyserver/certmagic/blob/master/handshake.go whose license(Apache 2.0) can be found here:        https://github.com/caddyserver/certmagic/blob/v0.16.1/LICENSE.txt

// createDevCertKey generates and saves(to disk) a certificate and key that can be used to configure a tls server.
//
// This is only meant to be used for development/local settings.
// The certificate is self-signed & a best effort is made to add its CA to the OS trust store.
// This function panics on error with the exception of adding certificate to OS trust store.
func createDevCertKey(l *slog.Logger) (certPath, keyPath string) {
	certPath, keyPath = certKeyPaths()
	rootCACertPath, _, err1 := rootCAcertKeyPaths()

	err2 := checkCertValidity(certPath)
	err3 := checkCertValidity(rootCACertPath)
	if err1 == nil && err2 == nil && err3 == nil {
		// certs exists and are valid.
		return certPath, keyPath
	}

	l.Info("creating dev tls cert and key")
	defer l.Info("done creating dev tls cert and key")

	if _, _, err := loadCA(); err != nil {
		l.Error("createDevCertKey", "error", err)
		panic(err)
	}

	caCert, caKey, err := installCA(l)
	if err != nil {
		// We should not panic for this error.
		// This is because this just represents a failure to add certs to CA store.
		e := errors.Wrap(err)
		l.Error("createDevCertKey", "error", e)
	}

	privKey, err := generatePrivKey()
	if err != nil {
		l.Error("createDevCertKey", "error", err)
		panic(err)
	}
	sg, ok := privKey.(crypto.Signer)
	if !ok {
		panic("privKey is not of type crypto.Signer")
	}
	pubKey := sg.Public()

	serNum, err := randomSerialNumber()
	if err != nil {
		l.Error("createDevCertKey", "error", err)
		panic(err)
	}

	certTemplate := &x509.Certificate{
		SerialNumber: serNum,
		Subject: pkix.Name{
			Organization:       []string{"ong development certificate"},
			OrganizationalUnit: []string{getOrg()},
		},
		DNSNames:  []string{"localhost"},
		NotBefore: time.Now().UTC(),
		// The maximum for `NotAfter` should be 825days
		// See https://support.apple.com/en-us/HT210176
		NotAfter:    time.Now().UTC().Add(26 * time.Hour),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}

	cert, err := x509.CreateCertificate(rand.Reader, certTemplate, caCert, pubKey, caKey)
	if err != nil {
		l.Error("createDevCertKey", "error", err)
		panic(err)
	}

	pemCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert})
	if err = os.WriteFile(certPath, pemCert, 0o644); err != nil {
		l.Error("createDevCertKey", "error", err)
		panic(err)
	}

	key, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		l.Error("createDevCertKey", "error", err)
		panic(err)
	}
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key})
	if err = os.WriteFile(keyPath, pemKey, 0o600); err != nil {
		l.Error("createDevCertKey", "error", err)
		panic(err)
	}

	return certPath, keyPath
}

// installCA adds the dev root CA to the linux/ubuntu certificate trust store.
func installCA(l *slog.Logger) (caCert *x509.Certificate, caKey any, err error) {
	l.Info("installing dev root CA")
	defer l.Info("done installing dev root CA")

	caCert, caKey, err = loadCA()
	if err != nil {
		return nil, nil, err
	}

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
		l.Info("root CA was already installed")
		return caCert, caKey, nil
	}

	rootCACertPath, _, err := rootCAcertKeyPaths()
	if err != nil {
		return nil, nil, errors.Wrap(err)
	}

	cert, err := os.ReadFile(rootCACertPath)
	if err != nil {
		return nil, nil, errors.Wrap(err)
	}

	cmd := commandWithSudo(l, "tee", systemTrustFilename())
	cmd.Stdin = bytes.NewReader(cert)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, nil, errors.Wrap(err)
	}

	sysTrustCmd := []string{"update-ca-certificates"}
	cmd = commandWithSudo(l, sysTrustCmd...)
	out, err = cmd.CombinedOutput()
	l.Info(string(out))
	if err != nil {
		return nil, nil, errors.Wrap(err)
	}

	installInNss := func() error {
		// certutil -V -d ~/.pki/nssdb -u L -n caUniqename # validate cert in NSS store.

		u, errC := user.Current()
		if errC != nil {
			return errors.Wrap(errC)
		}
		nssDb := filepath.Join(u.HomeDir, ".pki/nssdb")

		createDir := []string{"mkdir", "-p", nssDb}
		cmd = commandWithSudo(l, createDir...)
		out, err = cmd.CombinedOutput()
		l.Info(string(out), "args", cmd.Args, "error", err)
		if err != nil {
			return errors.Wrap(err)
		}

		delete := []string{"certutil", "-D", "-d", nssDb, "-n", caUniqename}
		cmd = commandWithSudo(l, delete...)
		out, err = cmd.CombinedOutput()
		l.Info(string(out), "args", cmd.Args, "error", err)
		_ = err // ignore error

		add := []string{"certutil", "-A", "-d", nssDb, "-t", "C,,", "-n", caUniqename, "-i", rootCACertPath}
		cmd = commandWithSudo(l, add...)
		out, err = cmd.CombinedOutput()
		l.Info(string(out), "args", cmd.Args)
		if err != nil {
			return errors.Wrap(err)
		}

		return nil
	}
	errNss := installInNss()

	return caCert, caKey, errNss
}

func loadCA() (caCert *x509.Certificate, caKey any, err error) {
	rootCACertPath, rootCAKeyPath, err := rootCAcertKeyPaths()
	if err != nil {
		return nil, nil, errors.Wrap(err)
	}

	if _, errSt := os.Stat(rootCACertPath); errSt != nil {
		if e := newCA(); e != nil {
			return nil, nil, e
		}
	}

	certPEMBlock, err := os.ReadFile(rootCACertPath)
	if err != nil {
		return nil, nil, errors.Wrap(err)
	}

	certDERBlock, _ := pem.Decode(certPEMBlock)
	if certDERBlock == nil || certDERBlock.Type != "CERTIFICATE" {
		return nil, nil, errors.New("failed to read CA cert")
	}

	caCert, err = x509.ParseCertificate(certDERBlock.Bytes)
	if err != nil {
		return nil, nil, errors.Wrap(err)
	}

	keyPEMBlock, err := os.ReadFile(rootCAKeyPath)
	if err != nil {
		return nil, nil, errors.Wrap(err)
	}

	keyDERBlock, _ := pem.Decode(keyPEMBlock)
	if keyDERBlock == nil || keyDERBlock.Type != "PRIVATE KEY" {
		return nil, nil, errors.New("failed to read CA key")
	}
	caKey, err = x509.ParsePKCS8PrivateKey(keyDERBlock.Bytes)
	if err != nil {
		return nil, nil, errors.Wrap(err)
	}

	return caCert, caKey, nil
}

func newCA() error {
	rootCACertPath, rootCAKeyPath, err := rootCAcertKeyPaths()
	if err != nil {
		return errors.Wrap(err)
	}

	privKey, err := generatePrivKey()
	if err != nil {
		return err
	}
	sg, ok := privKey.(crypto.Signer)
	if !ok {
		panic("privKey is not of type crypto.Signer")
	}
	pubKey := sg.Public()

	spkiASN1, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		return errors.Wrap(err)
	}

	var spki struct {
		Algorithm        pkix.AlgorithmIdentifier
		SubjectPublicKey asn1.BitString
	}
	_, err = asn1.Unmarshal(spkiASN1, &spki)
	if err != nil {
		return errors.Wrap(err)
	}

	serNum, err := randomSerialNumber()
	if err != nil {
		return errors.Wrap(err)
	}

	skid := sha1.Sum(spki.SubjectPublicKey.Bytes)
	tpl := &x509.Certificate{
		SerialNumber: serNum,
		Subject: pkix.Name{
			Organization:       []string{"ong development CA"},
			OrganizationalUnit: []string{getOrg()},
			// The CommonName is required by iOS to show the certificate in the
			// "Certificate Trust Settings" menu.
			// https://github.com/FiloSottile/mkcert/issues/47
			CommonName: "ong " + getOrg(),
		},
		SubjectKeyId:          skid[:],
		NotBefore:             time.Now().UTC(),
		NotAfter:              time.Now().UTC().AddDate(1, 0, 0), // 1year
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}

	cert, err := x509.CreateCertificate(rand.Reader, tpl, tpl, pubKey, privKey)
	if err != nil {
		return errors.Wrap(err)
	}

	privDER, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		return errors.Wrap(err)
	}

	err = os.WriteFile(
		rootCACertPath,
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert}),
		0o666,
	)
	if err != nil {
		return errors.Wrap(err)
	}

	err = os.WriteFile(
		rootCAKeyPath,
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER}),
		0o600,
	)
	if err != nil {
		return errors.Wrap(err)
	}

	return nil
}

func checkCertValidity(path string) error {
	certPEMBlock, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	certDERBlock, _ := pem.Decode(certPEMBlock)
	if certDERBlock == nil || certDERBlock.Type != "CERTIFICATE" {
		return err
	}

	cert, errX := x509.ParseCertificate(certDERBlock.Bytes)
	if errX != nil {
		return errX
	}

	now := time.Now().UTC()
	if now.Before(cert.NotBefore) || now.After(cert.NotAfter) {
		return errors.New("certificate date validity issue")
	}

	if len(cert.Issuer.Organization) != 1 {
		return errors.New("certificate issued by bad organization")
	}

	return nil
}

func commandWithSudo(l *slog.Logger, cmd ...string) *exec.Cmd {
	if u, err := user.Current(); err == nil && u.Uid == "0" {
		return exec.Command(cmd[0], cmd[1:]...)
	}

	cmdToRun := exec.Command("sudo", append([]string{"--prompt=Sudo password:", "--"}, cmd...)...)
	l.Info("commandWithSudo", "dir", cmdToRun.Dir, "path", cmdToRun.Path, "args", cmdToRun.Args)

	return cmdToRun
}

func rootCAcertKeyPaths() (string, string, error) {
	const (
		rootCACertName = "rootCA_cert.pem"
		rootCAKeyName  = "rootCA_key.pem"
		caRootPath     = "/tmp/ong"
	)

	_, err := os.Stat(caRootPath)
	if err != nil {
		if errMk := os.MkdirAll(caRootPath, 0o761); errMk != nil {
			return "", "", errors.Wrap(errMk)
		}
	}

	return filepath.Join(caRootPath, rootCACertName), filepath.Join(caRootPath, rootCAKeyName), nil
}

func certKeyPaths() (string, string) {
	const certPath = "/tmp/ong_dev_certificate.pem"
	const keyPath = "/tmp/ong_dev_key.pem"
	return certPath, keyPath
}

func generatePrivKey() (key crypto.PrivateKey, err error) {
	key, err = rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return key, nil
}

func randomSerialNumber() (*big.Int, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return serialNumber, nil
}

func getOrg() string {
	name, err := os.Hostname()
	if err == nil {
		name = "ong-org"
	}
	return name
}
