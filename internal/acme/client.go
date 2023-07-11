package acme

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/exp/slog"
)

// RFC 8555: https://datatracker.ietf.org/doc/html/rfc8555

// ACME flow:
// 0. Get directory        GET, 200
// 1. Get nonce.           HEAD, 200
// 2. Create account.      POST, 201, account
// 3. Submit order.        POST, 201, order
// 4. Fetch challenges     POST-GET, 200
// 5. Respond to challenges POST, 200
// 6. Poll status           POST-GET, 200
// 7. Finalize order        POST, 200
// 8. Poll status           POST-GET, 200
// 9. Download cert         POST-GET, 200

const (
	// ACME clients MUST send a User-Agent header field.
	// https://datatracker.ietf.org/doc/html/rfc8555#section-6.1
	userAgent = "name=ong/acme. version=v1. url=https://github.com/komuw/ong"
	//  ACME clients must have the Content-Type header field set to "application/jose+json"
	// https://datatracker.ietf.org/doc/html/rfc8555#section-6.2
	contentType = "application/jose+json"

	maxNumOfCertsInChain = 5
	maxCertSize          = 3072 * 4 // around 2048 when base64 encoded, multiply by 4 as a buffer.
	maxCertChainSize     = maxNumOfCertsInChain * maxCertSize
)

// getEcdsaPrivKey reads a private key from disk(or generates) one.
// It is used primarily to get;
// - An ACME account key.
// - A key to sign certificate signing requests.
func getEcdsaPrivKey(path string) (*ecdsa.PrivateKey, error) {
	// https://github.com/golang/crypto/blob/v0.10.0/acme/autocert/autocert.go#L936-L978

	{
		keyBytes, errA := os.ReadFile(path)
		if errA != nil {
			goto generate
		}

		priv, _ := pem.Decode(keyBytes)
		if priv == nil || !strings.Contains(priv.Type, "PRIVATE") {
			goto generate
		}

		pKey, errB := parsePrivateKey(priv.Bytes)
		if errB != nil {
			goto generate
		}
		return pKey, nil
	}

generate:
	{
		{
			pKey, errC := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			if errC != nil {
				return nil, errC
			}

			if errD := os.MkdirAll(filepath.Dir(path), 0o755); errD != nil {
				// If directory already exists, MkdirAll does nothing.
				return nil, errD
			}

			w, errE := os.OpenFile(
				path,
				// creates or truncates the named file.
				os.O_RDWR|os.O_CREATE|os.O_TRUNC,
				// rw - -
				0o600,
			)
			if errE != nil {
				return nil, errE
			}
			if errF := encodeECDSAKey(w, pKey); errF != nil {
				return nil, errF
			}

			return pKey, nil
		}
	}
}

// getDirectory is used to discover all the other relevant urls in an ACME server.
// https://datatracker.ietf.org/doc/html/rfc8555#section-7.1.1
func getDirectory(ctx context.Context, directoryURL string, l *slog.Logger) (directory, error) {
	d := directory{}

	ctx = context.WithValue(ctx, clientContextKey, "getDirectory")
	res, errA := getResponse(ctx, directoryURL, "GET", nil, l)
	if errA != nil {
		return d, errA
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		ae := &acmeError{}
		if errB := json.NewDecoder(res.Body).Decode(ae); errB != nil {
			return d, fmt.Errorf("ong/acme: unable to unmarshall directoryURL response : %w", errB)
		}

		return d, fmt.Errorf("ong/acme: get acme directoryURL failed: %w", ae)
	}

	if errC := json.NewDecoder(res.Body).Decode(&d); errC != nil {
		return d, fmt.Errorf("ong/acme: unable to unmarshall directoryURL response : %w", errC)
	}

	return d, nil
}

// getNonce returns an anti-replay token.
//
// In order to protect ACME resources from any possible replay attacks, ACME POST requests have a mandatory anti-replay mechanism.
// https://datatracker.ietf.org/doc/html/rfc8555#section-6.5
// https://datatracker.ietf.org/doc/html/rfc8555#section-7.2
func getNonce(ctx context.Context, newNonceURL string, l *slog.Logger) (string, error) {
	ctx = context.WithValue(ctx, clientContextKey, "getNonce")
	res, errA := getResponse(ctx, newNonceURL, "HEAD", nil, l)
	if errA != nil {
		return "", errA
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		ae := &acmeError{}
		if errB := json.NewDecoder(res.Body).Decode(ae); errB != nil {
			return "", fmt.Errorf("ong/acme: unable to unmarshall newNonceURL response : %w", errB)
		}

		return "", fmt.Errorf("ong/acme: get acme newNonceURL failed: %w", ae)
	}

	nonce := res.Header.Get("Replay-Nonce")
	if nonce == "" {
		return "", errors.New("ong/acme: nonce is invalid")
	}

	// The RFC says; "Clients MUST ignore invalid Replay-Nonce values."
	// https://datatracker.ietf.org/doc/html/rfc8555#section-6.5.1
	// todo: We wont do that now, but we should in future.

	return nonce, nil
}

// getAccount registers an account with ACME server.
// https://datatracker.ietf.org/doc/html/rfc8555#section-7.3
func getAccount(ctx context.Context, newAccountURL, newNonceURL, email string, accountPrivKey *ecdsa.PrivateKey, l *slog.Logger) (account, error) {
	/*
	   POST /acme/new-account HTTP/1.1
	   Content-Type: application/jose+json
	   {
	     "protected": base64url({
	       "alg": "ES256",
	       "jwk": {...},
	       "nonce": "6S8IqOGY7eL2lsGoTZYifg",
	       "url": "https://example.com/acme/new-account"
	     }),
	     "payload": base64url({
	       "termsOfServiceAgreed": true,
	       "contact": [
	         "mailto:cert-admin@example.org",
	         "mailto:admin@example.org"
	       ]
	     }),
	     "signature": "RZPOnYoPs1PhjszF...-nh6X1qtOFPB519I"
	   }
	*/

	actResponse := account{}
	actRequest := &account{Contact: []string{fmt.Sprintf("mailto:%s", email)}, TermsOfServiceAgreed: true}
	dataPayload, errA := json.Marshal(actRequest)
	if errA != nil {
		return actResponse, errA
	}

	bodyBytes, errB := prepBody(ctx, newAccountURL, newNonceURL, "", dataPayload, accountPrivKey, l)
	if errB != nil {
		return actResponse, errB
	}

	ctx = context.WithValue(ctx, clientContextKey, "getAccount")
	res, errC := getResponse(ctx, newAccountURL, "POST", bodyBytes, l)
	if errC != nil {
		return actResponse, errC
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode == http.StatusCreated || res.StatusCode == http.StatusOK {
		if errD := json.NewDecoder(res.Body).Decode(&actResponse); errD != nil {
			return actResponse, fmt.Errorf("ong/acme: unable to unmarshall newAccountURL response : %w", errD)
		}

		if !actResponse.TermsOfServiceAgreed {
			actResponse.TermsOfServiceAgreed = actRequest.TermsOfServiceAgreed
		}

		actResponse.kid = res.Header.Get("Location")
		return actResponse, nil
	}

	ae := &acmeError{}
	if errE := json.NewDecoder(res.Body).Decode(ae); errE != nil {
		return actResponse, fmt.Errorf("ong/acme: unable to unmarshall newAccountURL response : %w", errE)
	}

	return actResponse, fmt.Errorf("ong/acme: account creation failed: %w", ae)
}

// submitOrder starts certificate issuance process by requesting for an [order] from ACME.
// It returns the said order.
//
// The client begins the certificate issuance process by sending a POST request to the server's newOrder resource.
// In the order object, any authorization referenced in the "authorizations" array whose
// status is "pending" represents an authorization transaction that the client must complete before the server will issue the certificate
// https://datatracker.ietf.org/doc/html/rfc8555#section-7.4
func submitOrder(ctx context.Context, newOrderURL, newNonceURL, kid string, domains []string, accountPrivKey *ecdsa.PrivateKey, l *slog.Logger) (order, error) {
	/*
		POST /acme/new-order HTTP/1.1
		Content-Type: application/jose+json
		{
			"protected": base64url({
			  "alg": "ES256",
			  "kid": "https://example.com/acme/acct/evOfKhNU60wg",
			  "nonce": "5XJ1L3lEkMG7tR6pA00clA",
			  "url": "https://example.com/acme/new-order"
			}),
			"payload": base64url({
			"identifiers": [
				{ "type": "dns", "value": "www.example.org" },
				{ "type": "dns", "value": "example.org" }
			],
			"notBefore": "2016-01-01T00:04:00+04:00",
			"notAfter": "2016-01-08T00:04:00+04:00"
			}),
			"signature": "H6ZXtGjTZyUnPeKn...wEA4TklBdh3e454g"
		}
	*/

	orderResponse := order{}
	if len(domains) != 1 {
		// The `http-01` ACME challenge does not support wildcard,
		// So when we send submitOrder request, we expect to only send one domain.
		return orderResponse, fmt.Errorf("ong/acme: this client can only fetch a certificate for one domain at a time, found: %d", len(domains))
	}
	domain := domains[0]
	if strings.Contains(domain, "*") {
		// The `http-01` ACME challenge does not support wildcard,
		// So we also don't.
		return orderResponse, errors.New("ong/acme: this client does not currently support wildcard domain names")
	}
	identifiers := []identifier{{Type: "dns", Value: domain}}

	orderRequest := &order{Identifiers: identifiers}
	dataPayload, errA := json.Marshal(orderRequest)
	if errA != nil {
		return orderResponse, errA
	}

	bodyBytes, errB := prepBody(ctx, newOrderURL, newNonceURL, kid, dataPayload, accountPrivKey, l)
	if errB != nil {
		return orderResponse, errB
	}

	ctx = context.WithValue(ctx, clientContextKey, "submitOrder")
	res, errC := getResponse(ctx, newOrderURL, "POST", bodyBytes, l)
	if errC != nil {
		return orderResponse, errC
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode == http.StatusCreated {
		if errD := json.NewDecoder(res.Body).Decode(&orderResponse); errD != nil {
			return orderResponse, fmt.Errorf("ong/acme: unable to unmarshall newOrderURL response : %w", errD)
		}
		orderResponse.OrderURL = res.Header.Get("Location")

		return orderResponse, nil
	}

	ae := &acmeError{}
	if errE := json.NewDecoder(res.Body).Decode(ae); errE != nil {
		return orderResponse, fmt.Errorf("ong/acme: unable to unmarshall newOrderURL response : %w", errE)
	}

	return orderResponse, fmt.Errorf("ong/acme: order creation failed: %w", ae)
}

// fetchChallenges requests ACME server for a list of challenges that need to be fulfilled in order to get a certificate.
//
// In the order object, any authorization referenced in the "authorizations" array whose
// status is "pending" represents an authorization transaction that the client must complete before the server will issue the certificate
// https://datatracker.ietf.org/doc/html/rfc8555#section-7.4
//
// When a client receives an order from the server in reply to a newOrder request,
// it downloads the authorization resources by sending POST-as-GET requests to the indicated URLs.
// https://datatracker.ietf.org/doc/html/rfc8555#section-7.5
func fetchChallenges(ctx context.Context, authorizationURLS []string, newNonceURL, kid string, accountPrivKey *ecdsa.PrivateKey, l *slog.Logger) (authorization, error) {
	/*
	   POST /acme/authz/PAniVnsZcis HTTP/1.1
	   Content-Type: application/jose+json
	   {
	     "protected": base64url({
	       "alg": "ES256",
	       "kid": "https://example.com/acme/acct/evOfKhNU60wg",
	       "nonce": "uQpSjlRb4vQVCjVYAyyUWg",
	       "url": "https://example.com/acme/authz/PAniVnsZcis"
	     }),
	     "payload": "",
	     "signature": "nuSDISbWG8mMgE7H...QyVUL68yzf3Zawps"
	   }

	*/

	authorizationResponse := authorization{}

	if len(authorizationURLS) != 1 {
		// The `http-01` ACME challenge does not support wildcard,
		// So when we send submitOrder request, we expect to only send one domain and get back one authorization.
		return authorizationResponse, fmt.Errorf("ong/acme: expected only one authorization, found: %d", len(authorizationURLS))
	}

	authorizationURL := authorizationURLS[0]
	dataPayload := []byte("")

	bodyBytes, errA := prepBody(ctx, authorizationURL, newNonceURL, kid, dataPayload, accountPrivKey, l)
	if errA != nil {
		return authorizationResponse, errA
	}

	ctx = context.WithValue(ctx, clientContextKey, "fetchChallenges")
	res, errB := getResponse(ctx, authorizationURL, "POST", bodyBytes, l)
	if errB != nil {
		return authorizationResponse, errB
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode == http.StatusOK {
		if errC := json.NewDecoder(res.Body).Decode(&authorizationResponse); errC != nil {
			return authorizationResponse, fmt.Errorf("ong/acme: unable to unmarshall authorizationURL response : %w", errC)
		}

		http01Found := false
		for _, ch := range authorizationResponse.Challenges {
			if ch.Type == "http-01" {
				http01Found = true
				authorizationResponse.EffectiveChallenge = ch
			}
		}
		if !http01Found {
			return authorizationResponse, fmt.Errorf("ong/acme: authorizationResponse does not contain challenge type http-01. got: %v", authorizationResponse)
		}

		return authorizationResponse, nil
	}

	ae := &acmeError{}
	if errD := json.NewDecoder(res.Body).Decode(ae); errD != nil {
		return authorizationResponse, fmt.Errorf("ong/acme: unable to unmarshall authorizationURL response : %w", errD)
	}

	return authorizationResponse, fmt.Errorf("ong/acme: fetch challenges failed: %w", ae)
}

// checkChallengeStatus reports on the status of the ACME object found at url.
// It is called at;
// - start of respondToChallenge.
// https://datatracker.ietf.org/doc/html/rfc8555#section-7.5.1
func checkChallengeStatus(
	ctx context.Context,
	url string,
	newNonceURL string,
	kid string,
	accountPrivKey *ecdsa.PrivateKey,
	l *slog.Logger,
) (checkError error) {
	expectedStatus := "pending"
	chRes := checkStatusResp{}
	// This will sleep for a cumulative total time of 1m45secs.
	// https://go.dev/play/p/K2xsEgJ7eqW
	count := 0
	maxCount := 10
	dur := 10 * time.Nanosecond
	defer func() {
		if checkError != nil {
			l.Info("checkChallengeStatus",
				"url", url,
				"checkStatusResponse", chRes,
				"expectedStatus", expectedStatus,
				"count", count,
				"duration", dur,
				"error", checkError,
			)
		}
	}()

	dataPayload := []byte("")
	bodyBytes, errA := prepBody(ctx, url, newNonceURL, kid, dataPayload, accountPrivKey, l)
	if errA != nil {
		return errA
	}

	ctx = context.WithValue(ctx, clientContextKey, "checkChallengeStatus")
	for {
		if os.Getenv("ONG_RUNNING_IN_TESTS") != "" {
			l.InfoCtx(ctx, "checkStatusSleep",
				"count", count,
				"duration", dur,
				"checkError", checkError,
				"checkStatusResp", chRes,
				"expectedStatus", expectedStatus,
			)
			if count > 2 {
				panic("checkStatus is taking too long in tests")
			}
		}

		time.Sleep(dur)
		dur = dur + (2 * time.Second)
		count = count + 1
		if count >= maxCount {
			break
		}

		res, errB := getResponse(ctx, url, "GET", bodyBytes, l)
		if errB != nil {
			checkError = errB
			continue
		}
		defer func() { _ = res.Body.Close() }()
		dur = retryAfter(res.Header.Get("Retry-After"), dur) + (2 * time.Second)

		if res.StatusCode != http.StatusOK {
			ae := &acmeError{}
			if errC := json.NewDecoder(res.Body).Decode(ae); errC != nil {
				checkError = fmt.Errorf("ong/acme: unable to unmarshall checkAuthorizationStatus response : %w", errC)
				continue
			}

			checkError = ae
			continue
		}

		if errD := json.NewDecoder(res.Body).Decode(&chRes); errD != nil {
			checkError = errD
			continue
		}

		if strings.EqualFold(chRes.Status, expectedStatus) {
			checkError = nil
			break
		}
	}

	return checkError
}

// respondToChallenge informs ACME server that the challenges have been responded to.
// After this call, the ACME server will try to verify whether this is true.
// This should only be called if the challenges have been responded to,
// eg; by setting up a token on the `.well-known/acme-challenge` URI in case of http-01 challenge.
//
// To prove control of the identifier and receive authorization, the client needs to provision the required challenge response based on
// the challenge type and indicate to the server that it is ready for the challenge validation to be attempted.
// https://datatracker.ietf.org/doc/html/rfc8555#section-7.5.1
func respondToChallenge(ctx context.Context, ch challenge, newNonceURL, kid string, accountPrivKey *ecdsa.PrivateKey, l *slog.Logger) (challenge, error) {
	/*
		POST /acme/chall/prV_B7yEyA4 HTTP/1.1
		Content-Type: application/jose+json
		{
			"protected": base64url({
			  "alg": "ES256",
			  "kid": "https://example.com/acme/acct/evOfKhNU60wg",
			  "nonce": "Q_s3MWoqT05TrdkM2MTDcw",
			  "url": "https://example.com/acme/chall/prV_B7yEyA4"
			}),
			"payload": base64url({}),
			"signature": "9cbg5JO1Gf5YLjjz...SpkUfcdPai9uVYYQ"
		}
	*/

	challengeResponse := challenge{}
	if errA := checkChallengeStatus(ctx, ch.Url, newNonceURL, kid, accountPrivKey, l); errA != nil {
		return challengeResponse, errA
	}

	data := struct{}{}
	dataPayload, errB := json.Marshal(data)
	if errB != nil {
		return challengeResponse, errB
	}

	bodyBytes, errC := prepBody(ctx, ch.Url, newNonceURL, kid, dataPayload, accountPrivKey, l)
	if errC != nil {
		return challengeResponse, errC
	}

	ctx = context.WithValue(ctx, clientContextKey, "respondToChallenge")
	res, errD := getResponse(ctx, ch.Url, "POST", bodyBytes, l)
	if errD != nil {
		return challengeResponse, errD
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode == http.StatusOK {
		if errE := json.NewDecoder(res.Body).Decode(&challengeResponse); errE != nil {
			return challengeResponse, fmt.Errorf("ong/acme: unable to unmarshall challengeURL response : %w", errE)
		}

		return challengeResponse, nil
	}

	ae := &acmeError{}
	if errF := json.NewDecoder(res.Body).Decode(ae); errF != nil {
		return challengeResponse, fmt.Errorf("ong/acme: unable to unmarshall challengeURL response : %w", errF)
	}

	return challengeResponse, fmt.Errorf("ong/acme: respond to challenge failed: %w", ae)
}

// checkOrderStatus reports on the status of the ACME object found at url.
// It is called at;
// - start of sendCSR.
// - start of downloadCertificate.
// https://datatracker.ietf.org/doc/html/rfc8555#section-7.5.1
func checkOrderStatus(
	ctx context.Context,
	url string,
	expectedStatus string,
	newNonceURL string,
	kid string,
	accountPrivKey *ecdsa.PrivateKey,
	l *slog.Logger,
) (_ order, checkError error) {
	/*
		The status of the order will indicate what action the client should take:
		- "invalid": The certificate will not be issued. Consider this order process abandoned.
		- "pending": The server does not believe that the client has fulfilled the requirements.
					 Check the "authorizations" array for entries that are still pending.
		- "ready": The server agrees that the requirements have been fulfilled, and is awaiting finalization.
				   Submit a finalization request.
		- "processing": The certificate is being issued. Send a POST-as-GET request after the time given in the Retry-After header field of the response, if any.
		- "valid": The server has issued the certificate and provisioned its URL to the "certificate" field of the order. Download the certificate.

		https://datatracker.ietf.org/doc/html/rfc8555#section-7.4
	*/
	orderResponse := order{}
	// This will sleep for a cumulative total time of 3m00secs.
	// https://go.dev/play/p/K2xsEgJ7eqW
	count := 0
	maxCount := 10
	dur := 10 * time.Nanosecond
	start := time.Now().UTC()
	defer func() {
		if checkError != nil {
			l.Info("checkOrderStatus",
				"url", url,
				"orderResponse", orderResponse,
				"expectedStatus", expectedStatus,
				"count", count,
				"duration", dur,
				"totalDuration", time.Now().UTC().Sub(start),
				"error", checkError,
			)
		}
	}()

	dataPayload := []byte("")
	bodyBytes, errA := prepBody(ctx, url, newNonceURL, kid, dataPayload, accountPrivKey, l)
	if errA != nil {
		return orderResponse, errA
	}

	ctx = context.WithValue(ctx, clientContextKey, fmt.Sprintf("checkOrderStatus-%s", expectedStatus))
	for {
		if os.Getenv("ONG_RUNNING_IN_TESTS") != "" {
			l.InfoCtx(ctx, "checkStatusSleep",
				"count", count,
				"duration", dur,
				"checkError", checkError,
				"orderResponse", orderResponse,
				"expectedStatus", expectedStatus,
			)
			if count > 2 {
				panic("checkStatus is taking too long in tests")
			}
		}

		time.Sleep(dur)
		dur = dur + (4 * time.Second)
		count = count + 1
		if count >= maxCount {
			break
		}

		res, errB := getResponse(ctx, url, "GET", bodyBytes, l)
		if errB != nil {
			checkError = errB
			continue
		}
		defer func() { _ = res.Body.Close() }()
		dur = retryAfter(res.Header.Get("Retry-After"), dur) + (2 * time.Second)

		if res.StatusCode != http.StatusOK {
			ae := &acmeError{}
			if errC := json.NewDecoder(res.Body).Decode(ae); errC != nil {
				checkError = fmt.Errorf("ong/acme: unable to unmarshall checkAuthorizationStatus response : %w", errC)
				continue
			}

			checkError = ae
			continue
		}

		if errD := json.NewDecoder(res.Body).Decode(&orderResponse); errD != nil {
			checkError = errD
			continue
		}

		if strings.EqualFold(orderResponse.Status, expectedStatus) {
			checkError = nil
			break
		}
	}

	return orderResponse, checkError
}

// sendCSR sends a certificate signing request to acme.
//
// Once the client believes it has fulfilled the server's requirements,
// it should send a POST request to the order resource's finalize URL.
// The POST body MUST include a CSR.
// The CSR MUST indicate the exact same set of requested identifiers as the initial newOrder request.
// If a request to finalize an order is successful, the server will return a 200 (OK) with an updated order object.
// https://datatracker.ietf.org/doc/html/rfc8555#section-7.4
func sendCSR(ctx context.Context, domain string, o order, newNonceURL, kid string, accountPrivKey, certPrivKey *ecdsa.PrivateKey, l *slog.Logger) (orderRes order, _ error) {
	/*
		POST /acme/order/TOlocE8rfgo/finalize HTTP/1.1
		Content-Type: application/jose+json
		{
			"protected": base64url({
			  "alg": "ES256",
			  "kid": "https://example.com/acme/acct/evOfKhNU60wg",
			  "nonce": "MSF2j2nawWHPxxkE3ZJtKQ",
			  "url": "https://example.com/acme/order/TOlocE8rfgo/finalize"
			}),
			"payload": base64url({
			  "csr": "MIIBPTCBxAIBADBFMQ...FS6aKdZeGsysoCo4H9P",
			}),
			"signature": "uOrUfIIk5RyQ...nw62Ay1cl6AB"
		}

		csr is A CSR encoding the parameters for the certificate being requested [RFC2986].
		The CSR is sent in the base64url-encoded version of the DER format.
		(Note: Because this field uses base64url, and does not include headers, it is different from PEM.)
	*/

	// We won't check the challenge state(ch.Url) at this point. Although it should also be in valid state.
	//
	// A request to finalize an order will result in error if the order is not in the "ready" state.
	// - https://datatracker.ietf.org/doc/html/rfc8555#section-7.4
	orderURL := o.OrderURL
	defer func() {
		// `order.OrderURL` is not an ACME property.
		// This means that when json.Decode(&order) happens in this function,
		// the resulting order has no `OrderURL`; wo we have to add it back.
		orderRes.OrderURL = orderURL
	}()

	updatedO, errA := checkOrderStatus(ctx, orderURL, "ready", newNonceURL, kid, accountPrivKey, l)
	if errA != nil {
		return o, errA
	}
	o = updatedO
	o.OrderURL = orderURL

	req := &x509.CertificateRequest{
		Subject:  pkix.Name{CommonName: domain},
		DNSNames: []string{domain},
	}

	csrBytes, errB := x509.CreateCertificateRequest(rand.Reader, req, certPrivKey)
	if errB != nil {
		return o, errB
	}

	c := &csr{CSR: base64.RawURLEncoding.EncodeToString(csrBytes)}
	dataPayload, errC := json.Marshal(c)
	if errC != nil {
		return o, errC
	}

	url := o.FinalizeURL
	bodyBytes, errD := prepBody(ctx, url, newNonceURL, kid, dataPayload, accountPrivKey, l)
	if errD != nil {
		return o, errD
	}

	ctx = context.WithValue(ctx, clientContextKey, "sendCSR")
	res, errE := getResponse(ctx, url, "POST", bodyBytes, l)
	if errE != nil {
		return o, errE
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode == http.StatusOK {
		/*
			The status of the order will indicate what action the client should take:
			- "invalid": The certificate will not be issued. Consider this order process abandoned.
			- "pending": The server does not believe that the client has fulfilled the requirements.
						 Check the "authorizations" array for entries that are still pending.
			- "ready": The server agrees that the requirements have been fulfilled, and is awaiting finalization.
					   Submit a finalization request.
			- "processing": The certificate is being issued. Send a POST-as-GET request after the time given in the Retry-After header field of the response, if any.
			- "valid": The server has issued the certificate and provisioned its URL to the "certificate" field of the order. Download the certificate.

			https://datatracker.ietf.org/doc/html/rfc8555#section-7.4
		*/
		if errF := json.NewDecoder(res.Body).Decode(&o); errF != nil {
			return o, fmt.Errorf("ong/acme: unable to unmarshall finalizeURL response : %w", errF)
		}
		return o, nil
	}

	ae := &acmeError{}
	if errG := json.NewDecoder(res.Body).Decode(ae); errG != nil {
		return o, fmt.Errorf("ong/acme: unable to unmarshall finalizeURL response : %w", errG)
	}

	return o, fmt.Errorf("ong/acme: finalize failed: %w", ae)
}

// To download the issued certificate, the client simply sends a POST-as-GET request to the certificate URL.
// The default format of the certificate is application/pem-certificate-chain.
// A certificate resource represents a single, immutable certificate.
// If the client wishes to obtain a renewed certificate, the client initiates a new order process to request one.
// https://datatracker.ietf.org/doc/html/rfc8555#section-7.4.2
func downloadCertificate(ctx context.Context, o order, newNonceURL, kid string, accountPrivKey *ecdsa.PrivateKey, l *slog.Logger) (cert []byte, _ error) {
	/*
		POST /acme/cert/mAt3xBGaobw HTTP/1.1
		Content-Type: application/jose+json
		Accept: application/pem-certificate-chain
		{
			"protected": base64url({
			  "alg": "ES256",
			  "kid": "https://example.com/acme/acct/evOfKhNU60wg",
			  "nonce": "uQpSjlRb4vQVCjVYAyyUWg",
			  "url": "https://example.com/acme/cert/mAt3xBGaobw"
			}),
			"payload": "",
			"signature": "nuSDISbWG8mMgE7H...QyVUL68yzf3Zawps"
		}

		The status of the order will indicate what action the client should take:
		- "invalid": The certificate will not be issued. Consider this order process abandoned.
		- "pending": The server does not believe that the client has fulfilled the requirements.
						Check the "authorizations" array for entries that are still pending.
		- "ready": The server agrees that the requirements have been fulfilled, and is awaiting finalization.
					Submit a finalization request.
		- "processing": The certificate is being issued. Send a POST-as-GET request after the time given in the Retry-After header field of the response, if any.
		- "valid": The server has issued the certificate and provisioned its URL to the "certificate" field of the order. Download the certificate.

		https://datatracker.ietf.org/doc/html/rfc8555#section-7.4
	*/

	updatedO, errA := checkOrderStatus(ctx, o.OrderURL, "valid", newNonceURL, kid, accountPrivKey, l)
	if errA != nil {
		return nil, errA
	}
	updatedO.OrderURL = o.OrderURL
	certificateURL := updatedO.CertificateURL

	dataPayload := []byte("")

	bodyBytes, errB := prepBody(ctx, certificateURL, newNonceURL, kid, dataPayload, accountPrivKey, l)
	if errB != nil {
		return nil, errB
	}

	ctx = context.WithValue(ctx, clientContextKey, "downloadCertificate")
	res, errC := getResponse(ctx, certificateURL, "POST", bodyBytes, l)
	if errC != nil {
		return nil, errC
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode == http.StatusOK {
		c, errD := io.ReadAll(io.LimitReader(res.Body, maxCertChainSize))
		if errD != nil {
			return nil, errD
		}
		return c, nil
	}

	ae := &acmeError{}
	if errE := json.NewDecoder(res.Body).Decode(ae); errE != nil {
		return nil, fmt.Errorf("ong/acme: unable to unmarshall certificateDownload response : %w", errE)
	}

	return nil, fmt.Errorf("ong/acme: download certificate failed: %w", ae)
}
