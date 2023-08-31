package server

import (
	"crypto/tls"
	"errors"
	"fmt"

	"github.com/komuw/ong/config"
	"github.com/komuw/ong/internal/acme"
)

// Most of the code here is inspired(or taken from) by:
//   (a) https://github.com/eliben/code-for-blog whose license(Unlicense) can be found here:                                   https://github.com/eliben/code-for-blog/blob/464a32f686d7646ba3fc612c19dbb550ec8a05b1/LICENSE
//   (b) https://github.com/FiloSottile/mkcert   whose license(BSD 3-Clause ) can be found here:                               https://github.com/FiloSottile/mkcert/blob/v1.4.4/LICENSE
//   (c) https://github.com/golang/crypto/blob/master/acme/autocert/autocert.go whose license(BSD 3-Clause) can be found here: https://github.com/golang/crypto/blob/05595931fe9d3f8894ab063e1981d28e9873e2cb/LICENSE
//   (d) https://github.com/caddyserver/certmagic/blob/master/handshake.go whose license(Apache 2.0) can be found here:        https://github.com/caddyserver/certmagic/blob/v0.16.1/LICENSE.txt
//

// getTlsConfig returns a proper tls configuration given the options passed in.
// The tls config may either procure certificates from ACME, from disk or be nil(for non-tls traffic)
//
// h is the fallback is the http handler that will be delegated to for non ACME requests.
func getTlsConfig(o config.Opts) (c *tls.Config, e error) {
	if err := acme.Validate(o.Tls.Domain); err != nil {
		return nil, err
	}

	if o.Tls.AcmeEmail == "" && o.Tls.CertFile == "" && o.Tls.ClientCertificatePool != nil {
		return nil, errors.New("ong/server: clientCertificatePool cannot be specified if acmeEmail or certFile is unspecified")
	}

	if o.Tls.AcmeEmail != "" {
		// 1. use ACME.
		//
		if o.Tls.AcmeDirectoryUrl == "" {
			return nil, errors.New("ong/server: acmeDirectoryUrl cannot be empty if acmeEmail is also specified")
		}

		// You need to call it once instead of per request.
		// See: https://github.com/komuw/ong/issues/296
		getCert := acme.GetCertificate(
			o.Tls.Domain,
			o.Tls.AcmeEmail,
			o.Tls.AcmeDirectoryUrl,
			o.Logger,
		)
		// Support for acme certificate manager needs to be added in three places:
		// (a) In http middlewares.
		// (b) In http server.
		// (c) In http multiplexer.
		tlsConf := &tls.Config{
			// taken from:
			// https://github.com/golang/crypto/blob/05595931fe9d3f8894ab063e1981d28e9873e2cb/acme/autocert/autocert.go#L228-L234
			NextProtos: []string{
				"h2", // enable HTTP/2
				"http/1.1",
				"acme-tls/1", // enable tls-alpn ACME challenges
			},
			GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
				// GetCertificate returns a Certificate based on the given ClientHelloInfo.
				// it is called if `tls.Config.Certificates` is empty.
				//
				p := setFingerprint(info)

				c, err := getCert(info)
				if err != nil {
					// This will be logged by `http.Server.ErrorLog`
					ef := fmt.Errorf(
						"ong/server: failed to get certificate from ACME. acmeDirectoryUrl=%s, domain=%s, tls.ClientHelloInfo.ServerName=%s, clientIP=%s, clientFingerPrint=%s, : %w",
						o.Tls.AcmeDirectoryUrl,
						o.Tls.Domain,
						info.ServerName,
						info.Conn.RemoteAddr(),
						p,
						err,
					)
					return nil, ef
				}

				return c, nil
			},
		}

		if o.Tls.ClientCertificatePool != nil {
			tlsConf.ClientAuth = tls.RequireAndVerifyClientCert
			tlsConf.ClientCAs = o.Tls.ClientCertificatePool
		}
		return tlsConf, nil
	}
	if o.Tls.CertFile != "" {
		// 2. get from disk.
		//
		if len(o.Tls.KeyFile) < 1 {
			return nil, errors.New("ong/server: keyFile cannot be empty if certFile is also specified")
		}
		c, err := tls.LoadX509KeyPair(o.Tls.CertFile, o.Tls.KeyFile)
		if err != nil {
			return nil, err
		}

		tlsConf := &tls.Config{
			NextProtos: []string{
				"h2", // enable HTTP/2
				"http/1.1",
				"acme-tls/1", // enable tls-alpn ACME challenges
			},
			GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
				// GetCertificate returns a Certificate based on the given ClientHelloInfo.
				// it is called if `tls.Config.Certificates` is empty.
				//
				_ = setFingerprint(info)

				return &c, nil
			},
		}

		if o.Tls.ClientCertificatePool != nil {
			tlsConf.ClientAuth = tls.RequireAndVerifyClientCert
			tlsConf.ClientCAs = o.Tls.ClientCertificatePool
		}
		return tlsConf, nil
	}

	// 3. non-tls traffic.
	return nil, errors.New("ong/server: ong only serves https")
}
