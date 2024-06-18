package acme

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"go.akshayshah.org/attest"
)

const acmeTestToken = "random-unique-token-EuKDOWlre4"

// someAcmeServerHandler mimics an ACME server.
func someAcmeServerHandler(t *testing.T, domain string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if strings.HasPrefix(path, "/directory") {
			d := &directory{
				NewNonceURL:   fmt.Sprintf("http://%s/acme/new-nonce", r.Host),
				NewAccountURL: fmt.Sprintf("http://%s/acme/new-acct", r.Host),
				NewOrderURL:   fmt.Sprintf("http://%s/acme/new-order", r.Host),
			}

			w.WriteHeader(http.StatusOK)
			err := json.NewEncoder(w).Encode(d)
			attest.Ok(t, err)
			return
		}

		// getNonce
		if strings.HasPrefix(path, "/acme/new-nonce") {
			// Changing the header map after a call to WriteHeader (or Write) has no effect.
			// So, `WriteHeader` should be called last.
			// Otherwise, for the others; `WriteHeader` should be called first.
			//
			// If WriteHeader is not called explicitly, the first call to Write
			// will trigger an implicit WriteHeader(http.StatusOK).
			w.Header().Set("Replay-Nonce", "some-random-unique-nonce")
			w.WriteHeader(http.StatusOK)
			return
		}

		// getAccount
		if strings.HasPrefix(path, "/acme/new-acct") {
			a := &account{
				Status:               "valid",
				Contact:              []string{"mailto:hey+sample@gmail.com"},
				TermsOfServiceAgreed: true,
			}
			kid := fmt.Sprintf("http://%s/acme/acct/108401064", r.Host)
			w.Header().Set("Location", kid)

			w.WriteHeader(http.StatusCreated)
			err := json.NewEncoder(w).Encode(a)
			attest.Ok(t, err)
			return
		}

		// submitOrder
		if strings.HasPrefix(path, "/acme/new-order") {
			o := &order{
				Identifiers:    []identifier{{Type: "dns", Value: "heya.com"}},
				Authorizations: []string{fmt.Sprintf("http://%s/acme/authz-v3/7051965644", r.Host)},
				Status:         "pending",
				FinalizeURL:    fmt.Sprintf("http://%s/acme/finalize/108495164/9441031984", r.Host),
			}

			w.Header().Set("Location", fmt.Sprintf("http://%s/acme/order/108707254/9463788554", r.Host))

			w.WriteHeader(http.StatusCreated)
			err := json.NewEncoder(w).Encode(o)
			attest.Ok(t, err)
			return
		}

		// fetchChallenges
		if strings.HasPrefix(path, "/acme/authz-v3") {
			a := &authorization{
				Identifier: identifier{Type: "dns", Value: "heya.com"},
				Status:     "pending",
				Challenges: []challenge{
					{
						Type:   "http-01",
						Url:    fmt.Sprintf("http://%s/acme/chall-v3/7052305704/iSRkIw", r.Host),
						Status: "pending",
						Token:  acmeTestToken,
					},
				},
				EffectiveChallenge: challenge{
					Type:   "http-01",
					Url:    fmt.Sprintf("http://%s/acme/chall-v3/7052305704/iSRkIw", r.Host),
					Status: "pending",
					Token:  acmeTestToken,
				},
			}

			w.WriteHeader(http.StatusOK)
			err := json.NewEncoder(w).Encode(a)
			attest.Ok(t, err)
			return
		}

		// respondToChallenge
		if strings.HasPrefix(path, "/acme/chall-v3") {
			c := &challenge{
				Type:   "http-01",
				Url:    fmt.Sprintf("http://%s/acme/chall-v3/7052305704/iSRkIw", r.Host),
				Status: "pending",
				Token:  acmeTestToken,
			}

			w.WriteHeader(http.StatusOK)
			err := json.NewEncoder(w).Encode(c)
			attest.Ok(t, err)
			return
		}

		// sendCSR
		if strings.HasPrefix(path, "/acme/finalize") {
			o := &order{
				Identifiers:    []identifier{{Type: "dns", Value: "heya.com"}},
				Authorizations: []string{fmt.Sprintf("http://%s/acme/authz-v3/7051965644", r.Host)},
				Status:         "valid",
				FinalizeURL:    fmt.Sprintf("http://%s/acme/finalize/108495164/9441031984", r.Host),
				// Added certificate download url.
				CertificateURL: fmt.Sprintf("http://%s/acme/cert/mAt3xBGaobw", r.Host),
				OrderURL:       fmt.Sprintf("http://%s/acme/order/108707254/9463788554", r.Host),
			}

			w.WriteHeader(http.StatusOK)
			err := json.NewEncoder(w).Encode(o)
			attest.Ok(t, err)
			return
		}

		// checkOrder
		if strings.HasPrefix(path, "/acme/order") {
			requestType := r.Header.Get("ONG-TEST-REQ-TYPE")
			status := "valid"
			if strings.Contains(requestType, "ready") {
				// Before calling sendCSR we check order status and we expect it to be ready.
				// Before calling downloadCertificate  we check order status and we expect it to be valid.
				status = "ready"
			}

			o := &order{
				Identifiers:    []identifier{{Type: "dns", Value: "heya.com"}},
				Authorizations: []string{fmt.Sprintf("http://%s/acme/authz-v3/7051965644", r.Host)},
				Status:         status,
				FinalizeURL:    fmt.Sprintf("http://%s/acme/finalize/108495164/9441031984", r.Host),
				// Added certificate download url.
				CertificateURL: fmt.Sprintf("http://%s/acme/cert/mAt3xBGaobw", r.Host),
				OrderURL:       fmt.Sprintf("http://%s/acme/order/108707254/9463788554", r.Host),
			}

			w.WriteHeader(http.StatusOK)
			err := json.NewEncoder(w).Encode(o)
			attest.Ok(t, err)
			return
		}

		// downloadCertificate
		if strings.HasPrefix(path, "/acme/cert") {
			w.WriteHeader(http.StatusOK)

			privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			attest.Ok(t, err)
			certDer := createX509Cert(t, domain, privKey)
			buf := &bytes.Buffer{}
			err = pem.Encode(buf, &pem.Block{Type: "CERTIFICATE", Bytes: certDer})
			attest.Ok(t, err)

			_, err = w.Write(buf.Bytes())
			attest.Ok(t, err)
			return
		}

		buf := &bytes.Buffer{}
		dumpDebugW(buf, r, &http.Response{})
		panic(fmt.Sprintf("unexpected request to: %s", buf))
	}
}

func TestAcmeFunctions(t *testing.T) {
	t.Parallel()

	l := slog.Default()
	ctx := context.Background()

	setup := func(t *testing.T, acmeServerURL, domain string) (directory, account, order, authorization, order, *ecdsa.PrivateKey, *ecdsa.PrivateKey) {
		cacheDir := diskCachedir()

		directoryUrl, err := url.JoinPath(acmeServerURL, "/directory")
		attest.Ok(t, err)
		dir, err := getDirectory(ctx, directoryUrl, l)
		attest.Ok(t, err)

		accountKey := filepath.Join(cacheDir, accountKeyFileName)
		accountPrivKey, err := getEcdsaPrivKey(accountKey)
		attest.Ok(t, err)

		certKeyPath := filepath.Join(cacheDir, domain, certKeyFileName)
		certPrivKey, err := getEcdsaPrivKey(certKeyPath)
		attest.Ok(t, err)

		actResponse, err := getAccount(ctx, dir.NewAccountURL, dir.NewNonceURL, "hey+sample@gmail.com", accountPrivKey, l)
		attest.Ok(t, err)

		domains := []string{"heya.com"}
		orderResponse, err := submitOrder(ctx, dir.NewOrderURL, dir.NewNonceURL, actResponse.kid, domains, accountPrivKey, l)
		attest.Ok(t, err)

		authorizationResponse, err := fetchChallenges(ctx, orderResponse.Authorizations, dir.NewNonceURL, actResponse.kid, accountPrivKey, l)
		attest.Ok(t, err)

		updatedOrder, err := sendCSR(
			ctx,
			domain,
			orderResponse,
			dir.NewNonceURL,
			actResponse.kid,
			accountPrivKey,
			certPrivKey,
			l,
		)
		attest.Ok(t, err)

		return dir, actResponse, orderResponse, authorizationResponse, updatedOrder, accountPrivKey, certPrivKey
	}

	domain := getDomain()
	ts := httptest.NewServer(someAcmeServerHandler(t, domain))
	t.Cleanup(func() {
		ts.Close()
	})

	t.Run("getAccountKey", func(t *testing.T) {
		t.Parallel()

		accountKeyPath := t.TempDir()
		accountKey := filepath.Join(accountKeyPath, accountKeyFileName)

		got1, err := getEcdsaPrivKey(accountKey)
		attest.Ok(t, err)

		got2, err := getEcdsaPrivKey(accountKey)
		attest.Ok(t, err)

		attest.True(t, got1.Equal(got2))
	})

	t.Run("getDirectory", func(t *testing.T) {
		t.Parallel()

		directoryUrl, err := url.JoinPath(ts.URL, "/directory")
		attest.Ok(t, err)

		dir, err := getDirectory(ctx, directoryUrl, l)
		attest.Ok(t, err)
		attest.NotZero(t, dir.NewAccountURL)
		attest.NotZero(t, dir.NewNonceURL)
		attest.NotZero(t, dir.NewOrderURL)
	})

	t.Run("getNonce", func(t *testing.T) {
		t.Parallel()

		dir, _, _, _, _, _, _ := setup(t, ts.URL, domain)
		nonce, err := getNonce(ctx, dir.NewNonceURL, l)
		attest.Ok(t, err)

		attest.NotZero(t, nonce)
	})

	t.Run("getAccount", func(t *testing.T) {
		t.Parallel()

		dir, _, _, _, _, accountPrivKey, _ := setup(t, ts.URL, domain)
		actResponse, err := getAccount(
			ctx,
			dir.NewAccountURL,
			dir.NewNonceURL,
			"hey+sample@gmail.com",
			accountPrivKey,
			l)
		attest.Ok(t, err)

		attest.NotZero(t, actResponse.Status)
		attest.NotZero(t, actResponse.Contact)
		attest.NotZero(t, actResponse.TermsOfServiceAgreed)
		attest.NotZero(t, actResponse.kid)
		attest.Zero(t, actResponse.Orders)
	})

	t.Run("submitOrder", func(t *testing.T) {
		t.Parallel()

		domains := []string{"heya.com"}
		dir, acct, _, _, _, accountPrivKey, _ := setup(t, ts.URL, domain)
		orderResponse, err := submitOrder(
			ctx,
			dir.NewOrderURL,
			dir.NewNonceURL,
			acct.kid,
			domains,
			accountPrivKey,
			l)
		attest.Ok(t, err)

		attest.NotZero(t, orderResponse.Identifiers)
		attest.NotZero(t, orderResponse.Authorizations)
		attest.NotZero(t, orderResponse.Status)
		attest.NotZero(t, orderResponse.FinalizeURL)
		attest.Zero(t, orderResponse.CertificateURL)
		attest.Zero(t, orderResponse.Error)
	})

	t.Run("fetchChallenges", func(t *testing.T) {
		t.Parallel()

		dir, acct, ord, _, _, accountPrivKey, _ := setup(t, ts.URL, domain)
		authorizationResponse, err := fetchChallenges(
			ctx,
			ord.Authorizations,
			dir.NewNonceURL,
			acct.kid,
			accountPrivKey,
			l)
		attest.Ok(t, err)

		attest.NotZero(t, authorizationResponse.Identifier)
		attest.NotZero(t, authorizationResponse.Status)
		attest.NotZero(t, authorizationResponse.Status)
		attest.NotZero(t, authorizationResponse.Challenges)
		attest.NotZero(t, authorizationResponse.EffectiveChallenge)
		attest.NotZero(t, authorizationResponse.EffectiveChallenge.Type)
		attest.NotZero(t, authorizationResponse.EffectiveChallenge.Url)
		attest.NotZero(t, authorizationResponse.EffectiveChallenge.Status)
		attest.NotZero(t, authorizationResponse.EffectiveChallenge.Token)
	})

	t.Run("respondToChallenge", func(t *testing.T) {
		t.Parallel()

		dir, acct, _, authz, _, accountPrivKey, _ := setup(t, ts.URL, domain)
		challengeResponse, err := respondToChallenge(
			ctx,
			authz.EffectiveChallenge,
			dir.NewNonceURL,
			acct.kid,
			accountPrivKey,
			l,
		)
		attest.Ok(t, err)

		attest.NotZero(t, challengeResponse.Type)
		attest.NotZero(t, challengeResponse.Url)
		attest.NotZero(t, challengeResponse.Status)
		attest.NotZero(t, challengeResponse.Token)
	})

	t.Run("sendCSR", func(t *testing.T) {
		t.Parallel()

		dir, acct, ord, _, _, accountPrivKey, certPrivKey := setup(t, ts.URL, domain)
		updatedOrder, err := sendCSR(
			ctx,
			domain,
			ord,
			dir.NewNonceURL,
			acct.kid,
			accountPrivKey,
			certPrivKey,
			l,
		)
		attest.Ok(t, err)
		attest.NotZero(t, updatedOrder.Identifiers)
		attest.NotZero(t, updatedOrder.Authorizations)
		attest.NotZero(t, updatedOrder.Status)
		attest.NotZero(t, updatedOrder.CertificateURL)
		attest.Equal(t, updatedOrder.Status, "valid")
		attest.Zero(t, updatedOrder.Error)
	})

	t.Run("downloadCertificate", func(t *testing.T) {
		t.Parallel()

		dir, acct, _, _, updatedOrder, accountPrivKey, _ := setup(t, ts.URL, domain)
		certBytes, err := downloadCertificate(
			ctx,
			updatedOrder,
			dir.NewNonceURL,
			acct.kid,
			accountPrivKey,
			l,
		)
		attest.Ok(t, err)
		attest.NotZero(t, certBytes)
		// attest.NotZero(t, updatedOrder.Authorizations)
		// attest.NotZero(t, updatedOrder.Status)
		// attest.NotZero(t, updatedOrder.Certificate)
		// attest.Equal(t, updatedOrder.Status, "valid")
		// attest.Zero(t, updatedOrder.Error)
	})
}

func TestRealAcme(t *testing.T) {
	t.Parallel()

	// Comment out this line when you need to run this test.
	t.Skip("This test calls a real acme staging server and so should only be used on demand.")

	l := slog.Default()
	ctx := context.Background()

	t.Run("acme", func(t *testing.T) {
		t.Parallel()

		email := "hey+sample@gmail.com"
		domain := getDomain()
		acmeDirectoryUrl := "https://acme-staging-v02.api.letsencrypt.org/directory"
		accountKey := filepath.Join(t.TempDir(), accountKeyFileName)
		certKeyPath := filepath.Join(t.TempDir(), domain, certKeyFileName)

		accountPrivKey, err := getEcdsaPrivKey(accountKey)
		t.Log("accountPrivKey: ", accountPrivKey, err)
		attest.Ok(t, err)

		certPrivKey, err := getEcdsaPrivKey(certKeyPath)
		t.Log("certPrivKey: ", certPrivKey, err)
		attest.Ok(t, err)

		dir, err := getDirectory(ctx, acmeDirectoryUrl, l)
		t.Log("getDirectory: ", dir, err)
		attest.Ok(t, err)

		actResponse, err := getAccount(ctx, dir.NewAccountURL, dir.NewNonceURL, email, accountPrivKey, l)
		t.Log("getAccount: ", actResponse, err)
		attest.Ok(t, err)

		domains := []string{domain}
		orderResponse, err := submitOrder(ctx, dir.NewOrderURL, dir.NewNonceURL, actResponse.kid, domains, accountPrivKey, l)
		t.Log("submitOrder: ", orderResponse, err)
		attest.Ok(t, err)

		authorizationResponse, err := fetchChallenges(ctx, orderResponse.Authorizations, dir.NewNonceURL, actResponse.kid, accountPrivKey, l)
		t.Log("fetchChallenges: ", authorizationResponse, err)
		attest.Ok(t, err)

		// - run a http server in the same node where the dns records of the domain resolve to.
		// - make the url `http://myDomain.com/.well-known/acme-challenge/<token>` accesible

		challengeResponse, err := respondToChallenge(ctx, authorizationResponse.EffectiveChallenge, dir.NewNonceURL, actResponse.kid, accountPrivKey, l)
		t.Log("respondToChallenge: ", challengeResponse, err)
		attest.Ok(t, err)

		ord, err := sendCSR(ctx, domain, orderResponse, dir.NewNonceURL, actResponse.kid, accountPrivKey, certPrivKey, l)
		attest.Ok(t, err)

		certBytes, err := downloadCertificate(ctx, ord, dir.NewNonceURL, actResponse.kid, accountPrivKey, l)
		attest.Ok(t, err)
		attest.NotZero(t, certBytes)

		buf := &bytes.Buffer{}
		err = encodeECDSAKey(buf, certPrivKey)
		attest.Ok(t, err)

		_, err = buf.Write(certBytes)
		attest.Ok(t, err)

		t.Log("certificate: ", buf.String())
		cert, err := certFromReader(buf)
		attest.Ok(t, err)
		attest.NotZero(t, cert)
		t.Log("cert: ", cert)
	})
}
