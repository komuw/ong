package config

import (
	"testing"

	"go.akshayshah.org/attest"
)

func TestValidateAllowedOrigins(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		allowedOrigins []string
		succeeds       bool
		errMsg         string
	}{
		{
			name:           "okay",
			allowedOrigins: []string{"http://a.com"},
			succeeds:       true,
			errMsg:         "",
		},
		{
			name:           "has slash url path",
			allowedOrigins: []string{"http://b.com/"},
			succeeds:       false,
			errMsg:         "should not contain url path",
		},
		{
			name:           "has url path",
			allowedOrigins: []string{"https://c.com/hello"},
			succeeds:       false,
			errMsg:         "should not contain url path",
		},
		{
			name:           "okay with port",
			allowedOrigins: []string{"https://d.com:8888"},
			succeeds:       true,
			errMsg:         "",
		},
		{
			name:           "custom scheme okay",
			allowedOrigins: []string{"hzzs://e.com"},
			succeeds:       true,
			errMsg:         "",
		},
		{
			name:           "missing scheme",
			allowedOrigins: []string{"f.com"},
			succeeds:       false,
			errMsg:         "scheme should not be empty",
		},
		{
			name:           "wildcard with others",
			allowedOrigins: []string{"https://g.com", "*"},
			succeeds:       false,
			errMsg:         "single wildcard should not be used together with other allowedOrigins",
		},
		{
			name:           "multiple wildcard",
			allowedOrigins: []string{"http://*h*.com"},
			succeeds:       false,
			errMsg:         "should not contain more than one wildcard",
		},
		{
			name:           "wildcard should be prefixed to host",
			allowedOrigins: []string{"http://i*.com"},
			succeeds:       false,
			errMsg:         "wildcard should be prefixed to host",
		},
		{
			// null origin should be rejected, see: https://jub0bs.com/posts/2023-02-08-fearless-cors/#do-not-support-the-null-origin
			name:           "null origin",
			allowedOrigins: []string{"null"},
			succeeds:       false,
			errMsg:         "scheme should not be empty",
		},
		{
			name:           "wildcard is okay",
			allowedOrigins: []string{"http://*j.com"},
			succeeds:       true,
			errMsg:         "",
		},
		{
			name:           "wildcard in different domains",
			allowedOrigins: []string{"http://*k.com", "http://*another.com"},
			succeeds:       true,
			errMsg:         "",
		},
		{
			name:           "one wildcard",
			allowedOrigins: []string{"*"},
			succeeds:       true,
			errMsg:         "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateAllowedOrigins(tt.allowedOrigins)
			if tt.succeeds {
				attest.Ok(t, err)
			} else {
				attest.Error(t, err)
				attest.Subsequence(t, err.Error(), tt.errMsg)
			}
		})
	}
}

func TestValidateAllowedMethods(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		allowedMethods []string
		succeeds       bool
		errMsg         string
	}{
		{
			name:           "bad",
			allowedMethods: []string{"TRACE", "GET"},
			succeeds:       false,
			errMsg:         "method is forbidden",
		},
		{
			name:           "good",
			allowedMethods: []string{"PUT", "pOWEr"},
			succeeds:       true,
			errMsg:         "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateAllowedMethods(tt.allowedMethods)
			if tt.succeeds {
				attest.Ok(t, err)
			} else {
				attest.Error(t, err)
				attest.Subsequence(t, err.Error(), tt.errMsg)
			}
		})
	}
}

func TestValidateAllowedRequestHeaders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		allowedHeaders []string
		succeeds       bool
		errMsg         string
	}{
		{
			name:           "bad",
			allowedHeaders: []string{"Trailer"},
			succeeds:       false,
			errMsg:         "header is forbidden",
		},
		{
			name:           "other bad",
			allowedHeaders: []string{"ConTent-LenGTh"},
			succeeds:       false,
			errMsg:         "header is forbidden",
		},
		{
			name:           "again bad",
			allowedHeaders: []string{"sec-sasa"},
			succeeds:       false,
			errMsg:         "header is forbidden",
		},
		{
			name:           "good",
			allowedHeaders: []string{"DushDush"},
			succeeds:       true,
			errMsg:         "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateAllowedRequestHeaders(tt.allowedHeaders)
			if tt.succeeds {
				attest.Ok(t, err)
			} else {
				attest.Error(t, err)
				attest.Subsequence(t, err.Error(), tt.errMsg)
			}
		})
	}
}

func TestValidateAllowCredentials(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		allowCredentials bool
		allowedOrigins   []string
		allowedMethods   []string
		allowedHeaders   []string
		succeeds         bool
		errMsg           string
	}{
		{
			name:             "one wildcard origin and credentials",
			allowCredentials: true,
			allowedOrigins:   []string{"*"},
			allowedMethods:   nil,
			allowedHeaders:   nil,
			succeeds:         false,
			errMsg:           "allowCredentials should not be used together with wildcard",
		},
		{
			name:             "credentials no wildcard origin",
			allowCredentials: true,
			allowedOrigins:   []string{"https://example.com"},
			allowedMethods:   nil,
			allowedHeaders:   nil,
			succeeds:         true,
			errMsg:           "",
		},
		{
			name:             "one wildcard method and credentials",
			allowCredentials: true,
			allowedOrigins:   nil,
			allowedMethods:   []string{"*"},
			allowedHeaders:   nil,
			succeeds:         false,
			errMsg:           "allowCredentials should not be used together with wildcard",
		},
		{
			name:             "one wildcard header and credentials",
			allowCredentials: true,
			allowedOrigins:   nil,
			allowedMethods:   nil,
			allowedHeaders:   []string{"*"},
			succeeds:         false,
			errMsg:           "allowCredentials should not be used together with wildcard",
		},
		{
			name:             "insecure http scheme",
			allowCredentials: true,
			allowedOrigins:   []string{"http://example.org"},
			allowedMethods:   nil,
			allowedHeaders:   nil,
			succeeds:         false,
			errMsg:           "allowCredentials should not be used together with origin that uses unsecure scheme",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateAllowCredentials(tt.allowCredentials, tt.allowedOrigins, tt.allowedMethods, tt.allowedHeaders)
			if tt.succeeds {
				attest.Ok(t, err)
			} else {
				attest.Error(t, err)
				attest.Subsequence(t, err.Error(), tt.errMsg)
			}
		})
	}
}
