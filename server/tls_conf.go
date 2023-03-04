package server

import (
	"context"
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/net/idna"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

// Most of the code here is inspired(or taken from) by:
//   (a) https://github.com/eliben/code-for-blog whose license(Unlicense) can be found here:                                   https://github.com/eliben/code-for-blog/blob/464a32f686d7646ba3fc612c19dbb550ec8a05b1/LICENSE
//   (b) https://github.com/FiloSottile/mkcert   whose license(BSD 3-Clause ) can be found here:                               https://github.com/FiloSottile/mkcert/blob/v1.4.4/LICENSE
//   (c) https://github.com/golang/crypto/blob/master/acme/autocert/autocert.go whose license(BSD 3-Clause) can be found here: https://github.com/golang/crypto/blob/05595931fe9d3f8894ab063e1981d28e9873e2cb/LICENSE
//   (d) https://github.com/caddyserver/certmagic/blob/master/handshake.go whose license(Apache 2.0) can be found here:        https://github.com/caddyserver/certmagic/blob/v0.16.1/LICENSE.txt
//   (e) https://github.com/bpowers/go-fingerprint-example whose license(ISC License) can be found here:                       https://github.com/bpowers/go-fingerprint-example/blob/d411f76d221249bd19085eb4baeff6f5c45b24c9/LICENSE
//   (f) https://github.com/sleeyax/ja3rp whose license(MIT) can be found here:                                                https://github.com/sleeyax/ja3rp/blob/v0.0.1/LICENSE
//

// getTlsConfig returns a proper tls configuration given the options passed in.
// The tls config may either procure certifiates from LetsEncrypt, from disk or be nil(for non-tls traffic)
func getTlsConfig(o Opts) (*tls.Config, error) {
	if err := validateDomain(o.tls.domain); err != nil {
		return nil, err
	}

	if o.tls.email != "" {
		// 1. use letsencrypt.
		//
		const letsEncryptProductionUrl = "https://acme-v02.api.letsencrypt.org/directory"
		const letsEncryptStagingUrl = "https://acme-staging-v02.api.letsencrypt.org/directory"

		url := letsEncryptProductionUrl
		if os.Getenv("ONG_RUNNING_IN_TESTS") != "" {
			url = letsEncryptStagingUrl
		}
		m := &autocert.Manager{
			Client:     &acme.Client{DirectoryURL: url},
			Cache:      autocert.DirCache("ong-certifiate-dir"),
			Prompt:     autocert.AcceptTOS,
			Email:      o.tls.email,
			HostPolicy: customHostWhitelist(o.tls.domain),
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
		if len(o.tls.keyFile) < 1 {
			return nil, errors.New("ong/server: keyFile cannot be empty if certFile is also specified")
		}
		c, err := tls.LoadX509KeyPair(o.tls.certFile, o.tls.keyFile)
		if err != nil {
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

				{
					getFingerprint := func(info *tls.ClientHelloInfo) string {
						// SSLVersion,Cipher,SSLExtension,EllipticCurve,EllipticCurvePointFormat

						// TODO: check if this table is upto date and accurate.
						greaseTable := map[uint16]bool{
							0x0a0a: true, 0x1a1a: true, 0x2a2a: true, 0x3a3a: true,
							0x4a4a: true, 0x5a5a: true, 0x6a6a: true, 0x7a7a: true,
							0x8a8a: true, 0x9a9a: true, 0xaaaa: true, 0xbaba: true,
							0xcaca: true, 0xdada: true, 0xeaea: true, 0xfafa: true,
						}

						s := ""
						ver := uint16(0)
						for _, v := range info.SupportedVersions {
							// TODO: explain this.
							// ja3 wants the version chosen, not the list of versions.
							// see: https://sourcegraph.com/github.com/golang/go@go1.19.4/-/blob/src/crypto/tls/handshake_client.go?L62-71
							if v > ver {
								ver = v
							}
						}
						s += fmt.Sprintf("%d,", ver)

						vals := []string{}
						for _, v := range info.CipherSuites {
							vals = append(vals, fmt.Sprintf("%d", v))
						}
						s += fmt.Sprintf("%s,", strings.Join(vals, "-"))

						// TODO: Explain this. Because `tls.ClientHelloInfo` does not have extensions.
						// This should be fixed if https://github.com/golang/go/issues/32936 is ever implemented.
						extensions := []uint16{}
						vals = []string{}
						for _, v := range extensions {
							if _, ok := greaseTable[v]; ok {
								continue
							}

							vals = append(vals, fmt.Sprintf("%d", v))
						}
						s += fmt.Sprintf("%s,", strings.Join(vals, "-"))

						vals = []string{}
						for _, v := range info.SupportedCurves {
							vals = append(vals, fmt.Sprintf("%d", v))
						}
						s += fmt.Sprintf("%s,", strings.Join(vals, "-"))

						vals = []string{}
						for _, v := range info.SupportedPoints {
							vals = append(vals, fmt.Sprintf("%d", v))
						}
						s += fmt.Sprintf("%s", strings.Join(vals, "-"))

						hasher := md5.New()
						hasher.Write([]byte(s))
						return hex.EncodeToString(hasher.Sum(nil))
					}

					if conn, ok := info.Conn.(*komuConn); ok {
						jHash := getFingerprint(info)
						conn.fingerprint.Load().Val.Store(&jHash)
					}
				}

				return &c, nil
			},
		}
		return tlsConf, nil
	}

	// 3. non-tls traffic.
	return nil, errors.New("ong/server: ong only serves https")
}

func validateDomain(domain string) error {
	if len(domain) < 1 {
		return errors.New("ong/server: domain cannot be empty")
	}
	if strings.Count(domain, "*") > 1 {
		return errors.New("ong/server: domain can only contain one wildcard character")
	}
	if strings.Contains(domain, "*") && !strings.HasPrefix(domain, "*") {
		return errors.New("ong/server: wildcard character should be a prefix")
	}
	if strings.Contains(domain, "*") && domain[1] != '.' {
		return errors.New("ong/server: wildcard character should be followed by a `.` character")
	}

	if !strings.Contains(domain, "*") {
		// not wildcard
		if _, err := idna.Lookup.ToASCII(domain); err != nil {
			return err
		}
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
