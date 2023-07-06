package acme

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"sync"
	"time"

	"golang.org/x/exp/slog"
)

type clientContextKeyType string

const (
	clientContextKey = clientContextKeyType("clientContextKey")
	urtp             = "unknownRequestType"
)

var (
	clientOnce sync.Once    //nolint:gochecknoglobals
	client     *http.Client //nolint:gochecknoglobals
)

func getRequestType(ctx context.Context) string {
	if ctx != nil {
		if vCtx := ctx.Value(clientContextKey); vCtx != nil {
			if s, ok := vCtx.(string); ok {
				return s
			}
		}
	}

	return urtp
}

// getHttpClient returns a [http.Client]. It creates and reuses http.Clients as needed.
func getHttpClient(timeout time.Duration, l *slog.Logger) *http.Client {
	clientOnce.Do(func() {
		dialer := &net.Dialer{
			// see: net.DefaultResolver
			Resolver: &net.Resolver{PreferGo: true}, // Prefer Go's built-in DNS resolver.
			// Maximum amount of time a dial will wait for a connect to complete.
			// see; http.DefaultTransport
			Timeout:   timeout,
			KeepAlive: 3 * timeout, // interval between keep-alive probes
		}
		t := &http.Transport{
			// see: http.DefaultTransport
			DialContext:           dialer.DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       5 * timeout,
			TLSHandshakeTimeout:   timeout,
			ExpectContinueTimeout: (timeout / 5),
		}

		ft := &logRT{t, l}

		client = &http.Client{Transport: ft, Timeout: timeout}
	})
	return client
}

// logRT is a [http.RoundTripper].
type logRT struct {
	*http.Transport
	l *slog.Logger
}

func (lt *logRT) RoundTrip(req *http.Request) (res *http.Response, err error) {
	ctx := req.Context()
	requestType := getRequestType(ctx)
	// todo: Gate this under testing.Testing()
	req.Header.Set("ONG-TEST-REQ-TYPE", requestType)
	start := time.Now()
	url := req.URL.Redacted()

	defer func() {
		// dumpDebugW(os.Stdout, req, res)

		msg := "ong_acme_http_client"
		flds := []any{
			"method", req.Method,
			"url", url,
			"requestType", requestType,
			"durationMS", time.Since(start).Milliseconds(),
		}

		if err != nil {
			extra := []any{"err", err}
			flds = append(flds, extra...)
			lt.l.ErrorCtx(ctx, msg, flds...)
		} else if res.StatusCode > 399 {
			extra := []any{
				"code", res.StatusCode,
				"status", res.Status,
			}
			flds = append(flds, extra...)
			lt.l.ErrorCtx(ctx, msg, flds...)
		}
	}()

	return lt.Transport.RoundTrip(req)
}

func getResponse(ctx context.Context, url, method string, body []byte, l *slog.Logger) (*http.Response, error) {
	var br io.Reader
	if len(body) != 0 {
		br = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, br)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Content-Type", contentType)

	httpClient := getHttpClient(3*time.Second, l)

	return httpClient.Do(req)
}

func prepBody(url, newNonceURL, kid string, dataPayload []byte, accountPrivKey *ecdsa.PrivateKey, l *slog.Logger) ([]byte, error) {
	nonce, err := getNonce(newNonceURL, l)
	if err != nil {
		return nil, err
	}

	var prot any
	alg, sha := jwsHasher(accountPrivKey.PublicKey)
	if kid == "" {
		jwk := jwkEncode(accountPrivKey.PublicKey)
		prot = &protected{
			Alg:   alg,
			Nonce: nonce,
			Url:   url,
			Jwk:   &jwk,
		}
	} else {
		k := new(string)
		*k = kid
		prot = &protected{
			Alg:   alg,
			Nonce: nonce,
			Url:   url,
			Kid:   k,
		}
	}

	protPayload, err := json.Marshal(prot)
	if err != nil {
		return nil, err
	}
	protPayloadStr := base64.RawURLEncoding.EncodeToString(protPayload)

	// Base64 encoding using the URL- and filename-safe character set (section 5 of RFC 4648) [RFC4648], with all trailing '=' characters omitted
	// https://datatracker.ietf.org/doc/html/rfc7515#section-2
	dataPayloadStr := base64.RawURLEncoding.EncodeToString(dataPayload)
	if len(dataPayload) == 0 {
		// When sending POST-as-GET requests, the body must be the empty string,
		// NOT the json encoding of the empty string.
		// https://datatracker.ietf.org/doc/html/rfc8555#section-6.3
		dataPayloadStr = ""
	}

	hash := sha.New()
	_, _ = io.WriteString(hash, protPayloadStr+"."+dataPayloadStr)
	sig, err := jwsSign(accountPrivKey, sha, hash.Sum(nil))
	if err != nil {
		return nil, err
	}
	sigStr := base64.RawURLEncoding.EncodeToString(sig)

	jws := &jsonWebSignature{
		Protected: protPayloadStr,
		Payload:   dataPayloadStr,
		Signature: sigStr,
	}

	jwsByte, err := json.Marshal(jws)
	if err != nil {
		return nil, err
	}

	if jwsByte == nil {
		return nil, fmt.Errorf("ong/acme: http body for request, %s, cannot be empty", url)
	}

	return jwsByte, nil
}

// dumpDebugW is used for debugging purposes.
func dumpDebugW(w io.Writer, req *http.Request, res *http.Response) {
	breq, err := httputil.DumpRequest(req, true)
	s := "\n\n===========================DUMPING REQUEST===========================\n"
	s = s + string(breq)
	if err != nil {
		s = s + fmt.Sprintf("\n\t dumpRequest error: %v", err)
	}
	s = s + "\n===========================DUMPING REQUEST===========================\n"

	bres, err := httputil.DumpResponse(res, true)
	s = s + "\n===========================DUMPING RESPONSE===========================\n"
	s = s + string(bres)
	if err != nil {
		s = s + fmt.Sprintf("\n\t dumpResponse error: %v", err)
	}
	s = s + "\n===========================DUMPING RESPONSE===========================\n\n"

	_, _ = io.WriteString(w, s)
}
