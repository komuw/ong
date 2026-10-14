package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.akshayshah.org/attest"
)

const secFetchSiteHeader = "Sec-Fetch-Site"

func someCsrfHandler(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, msg)
	}
}

func TestCsrf(t *testing.T) {
	t.Parallel()

	const domain = "example.com"

	tests := []struct {
		name               string
		method             string
		secFetchSite       string
		origin             string
		trustedCsrfOrigins []string
		wantStatus         int
	}{
		// safe methods.
		{name: "GET cross-site", method: http.MethodGet, secFetchSite: "cross-site", origin: "https://attacker.example", wantStatus: http.StatusOK},
		{name: "HEAD cross-site", method: http.MethodHead, secFetchSite: "cross-site", origin: "https://attacker.example", wantStatus: http.StatusOK},
		{name: "OPTIONS cross-site", method: http.MethodOptions, secFetchSite: "cross-site", origin: "https://attacker.example", wantStatus: http.StatusOK},

		// Sec-Fetch-Site.
		{name: "POST same-origin", method: http.MethodPost, secFetchSite: "same-origin", wantStatus: http.StatusOK},
		{name: "POST none", method: http.MethodPost, secFetchSite: "none", wantStatus: http.StatusOK},
		{name: "POST cross-site", method: http.MethodPost, secFetchSite: "cross-site", origin: "https://attacker.example", wantStatus: http.StatusForbidden},
		{name: "POST same-site", method: http.MethodPost, secFetchSite: "same-site", origin: "https://blog.example.com", wantStatus: http.StatusForbidden},
		{name: "PUT cross-site", method: http.MethodPut, secFetchSite: "cross-site", wantStatus: http.StatusForbidden},
		{name: "PATCH cross-site", method: http.MethodPatch, secFetchSite: "cross-site", wantStatus: http.StatusForbidden},
		{name: "DELETE cross-site", method: http.MethodDelete, secFetchSite: "cross-site", wantStatus: http.StatusForbidden},
		{name: "TRACE cross-site", method: http.MethodTrace, secFetchSite: "cross-site", wantStatus: http.StatusForbidden},

		// No Sec-Fetch-Site.
		{name: "POST not from browser", method: http.MethodPost, wantStatus: http.StatusOK},
		{name: "POST origin matches host", method: http.MethodPost, origin: "https://" + domain, wantStatus: http.StatusOK},
		{name: "POST origin does not match host", method: http.MethodPost, origin: "https://attacker.example", wantStatus: http.StatusForbidden},
		{name: "POST null origin", method: http.MethodPost, origin: "null", wantStatus: http.StatusForbidden},

		// trustedCsrfOrigins.
		{name: "www is not trusted by default", method: http.MethodPost, secFetchSite: "same-site", origin: "https://www." + domain, wantStatus: http.StatusForbidden},
		{name: "other subdomains are not trusted by default", method: http.MethodPost, secFetchSite: "same-site", origin: "https://blog." + domain, wantStatus: http.StatusForbidden},
		{name: "trusted origin", method: http.MethodPost, secFetchSite: "cross-site", origin: "https://example.net", trustedCsrfOrigins: []string{"https://example.net"}, wantStatus: http.StatusOK},
		{name: "trusted origin without Sec-Fetch-Site", method: http.MethodPost, origin: "https://example.net", trustedCsrfOrigins: []string{"https://example.net"}, wantStatus: http.StatusOK},
		{name: "untrusted origin", method: http.MethodPost, secFetchSite: "cross-site", origin: "https://attacker.example", trustedCsrfOrigins: []string{"https://example.net"}, wantStatus: http.StatusForbidden},
		{name: "subdomain of trusted origin is not trusted", method: http.MethodPost, secFetchSite: "same-site", origin: "https://app.example.net", trustedCsrfOrigins: []string{"https://example.net"}, wantStatus: http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			msg := "hello"
			wrappedHandler := csrf(someCsrfHandler(msg), tt.trustedCsrfOrigins)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, "https://"+domain+"/someUri", nil)
			if tt.secFetchSite != "" {
				req.Header.Set(secFetchSiteHeader, tt.secFetchSite)
			}
			if tt.origin != "" {
				req.Header.Set(originHeader, tt.origin)
			}
			wrappedHandler.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)

			attest.Equal(t, res.StatusCode, tt.wantStatus)
			if tt.wantStatus == http.StatusOK {
				attest.Equal(t, string(rb), msg)
				attest.Zero(t, res.Header.Get(ongMiddlewareErrorHeader))
			} else {
				attest.NotEqual(t, string(rb), msg)
				attest.NotZero(t, res.Header.Get(ongMiddlewareErrorHeader))
			}
		})
	}

	t.Run("bad allowedOrigin panics", func(t *testing.T) {
		t.Parallel()

		attest.Panics(t, func() {
			_ = csrf(someCsrfHandler("hello"), []string{"https://example.net/some/path"})
		})
	})
}
