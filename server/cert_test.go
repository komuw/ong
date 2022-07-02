package server

import (
	"os"
	"testing"

	"github.com/akshayjshah/attest"
)

func TestCreateDevCertKey(t *testing.T) {
	t.Parallel()

	if os.Getenv("GITHUB_ACTIONS") != "" {
		// setRlimit() fails in github actions with error: `operation not permitted`
		// specifically the call to `unix.Setrlimit()`
		return
	}

	certPath, keyPath := certKeyPaths()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		os.Remove(certPath)
		os.Remove(keyPath)

		_, _ = CreateDevCertKey()

		_, err := os.Stat(certPath)
		attest.Ok(t, err)

		_, err = os.Stat(keyPath)
		attest.Ok(t, err)
	})
}

// taken from  https://github.com/golang/crypto/blob/05595931fe9d3f8894ab063e1981d28e9873e2cb/acme/autocert/autocert_test.go#L672
func TestCustomHostWhitelist(t *testing.T) {
	t.Parallel()

	// policy := customHostWhitelist("example.com", "EXAMPLE.ORG", "*.example.net", "éÉ.com")
	// tt := []struct {
	// 	host  string
	// 	allow bool
	// }{
	// 	{"example.com", true},
	// 	{"example.org", true},
	// 	{"xn--9caa.com", true}, // éé.com
	// 	{"one.example.com", false},
	// 	{"two.example.org", false},
	// 	{"three.example.net", false},
	// 	{"dummy", false},
	// }
	// for i, test := range tt {
	// 	err := policy(nil, test.host)
	// 	if err != nil && test.allow {
	// 		t.Errorf("%d: policy(%q): %v; want nil", i, test.host, err)
	// 	}
	// 	if err == nil && !test.allow {
	// 		t.Errorf("%d: policy(%q): nil; want an error", i, test.host)
	// 	}
	// }
	//

	t.Run("one domain", func(t *testing.T) {
		t.Parallel()

		policy := customHostWhitelist("example.com")
		tt := []struct {
			host  string
			allow bool
		}{
			{"example.com", true},
			{"example.org", false},
			{"xn--9caa.com", false}, // éé.com
			{"one.example.com", false},
			{"two.example.org", false},
			{"three.example.net", false},
			{"dummy", false},
		}
		for i, test := range tt {
			err := policy(nil, test.host)
			if err != nil && test.allow {
				t.Errorf("%d: policy(%q): %v; want nil", i, test.host, err)
			}
			if err == nil && !test.allow {
				t.Errorf("%d: policy(%q): nil; want an error", i, test.host)
			}
		}
	})

	t.Run("Punycode", func(t *testing.T) {
		t.Parallel()

		policy := customHostWhitelist("éÉ.com")
		tt := []struct {
			host  string
			allow bool
		}{
			{"example.com", false},
			{"example.org", false},
			{"xn--9caa.com", true}, // éé.com
			{"one.example.com", false},
			{"two.example.org", false},
			{"three.example.net", false},
			{"dummy", false},
		}
		for i, test := range tt {
			err := policy(nil, test.host)
			if err != nil && test.allow {
				t.Errorf("%d: policy(%q): %v; want nil", i, test.host, err)
			}
			if err == nil && !test.allow {
				t.Errorf("%d: policy(%q): nil; want an error", i, test.host)
			}
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		t.Parallel()

		policy := customHostWhitelist("EXAMPLE.ORG")
		tt := []struct {
			host  string
			allow bool
		}{
			{"example.com", false},
			{"example.org", true},
			{"xn--9caa.com", false}, // éé.com
			{"one.example.com", false},
			{"two.example.org", false},
			{"three.example.net", false},
			{"dummy", false},
		}
		for i, test := range tt {
			err := policy(nil, test.host)
			if err != nil && test.allow {
				t.Errorf("%d: policy(%q): %v; want nil", i, test.host, err)
			}
			if err == nil && !test.allow {
				t.Errorf("%d: policy(%q): nil; want an error", i, test.host)
			}
		}
	})

	t.Run("wildcard lowercase", func(t *testing.T) {
		t.Parallel()

		policy := customHostWhitelist("*.example.com")
		tt := []struct {
			host  string
			allow bool
		}{
			{"example.com", true},
			{"EXAMPLE.COM", true},
			{"www.example.com", true},
			{"WWW.EXAMPLE.COM", true},
			{"WWW.example.COM", true},
			//
			{"example.org", false},
			{"xn--9caa.com", false}, // éé.com
			//
			{"one.example.com", true},
			{"alas.example.com", true},
			{"alas.EXAMPLE.com", true},
			{"ALAS.EXAMPLE.COM", true},
			{"abc.def.example.com", false},
			//
			{"two.example.org", false},
			{"three.example.net", false},
			{"dummy", false},
		}
		for i, test := range tt {
			err := policy(nil, test.host)
			if err != nil && test.allow {
				t.Errorf("%d: policy(%q): %v; want nil", i, test.host, err)
			}
			if err == nil && !test.allow {
				t.Errorf("%d: policy(%q): nil; want an error", i, test.host)
			}
		}
	})

	t.Run("wildcard uppercase", func(t *testing.T) {
		t.Parallel()

		policy := customHostWhitelist("*.EXAMPLE.COM")
		tt := []struct {
			host  string
			allow bool
		}{
			{"example.com", true},
			{"EXAMPLE.COM", true},
			{"www.example.com", true},
			{"WWW.EXAMPLE.COM", true},
			{"WWW.example.COM", true},
			//
			{"example.org", false},
			{"xn--9caa.com", false}, // éé.com
			//
			{"one.example.com", true},
			{"alas.example.com", true},
			{"alas.EXAMPLE.com", true},
			{"ALAS.EXAMPLE.COM", true},
			{"abc.def.example.com", false},
			//
			{"two.example.org", false},
			{"three.example.net", false},
			{"dummy", false},
		}
		for i, test := range tt {
			err := policy(nil, test.host)
			if err != nil && test.allow {
				t.Errorf("%d: policy(%q): %v; want nil", i, test.host, err)
			}
			if err == nil && !test.allow {
				t.Errorf("%d: policy(%q): nil; want an error", i, test.host)
			}
		}
	})
}

func TestValidateDomain(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		tt := []struct {
			domain       string
			shouldSucced bool
		}{
			{"example.com", true},
			{"example.org", true},
			{"xn--9caa.com", true}, // éé.com
			{"one.example.com", true},
			//
			{"*.example.org", true},
			{"*.example.*", false},
			{"example.*org", false},
			//
			{"", false},
			{"exampl_e.com", false}, // underscore(rune U+005F) is disallowed in domain names.
			//
			{"dummy", true},
		}
		for _, test := range tt {
			err := validateDomain(test.domain)
			if test.shouldSucced {
				attest.Ok(t, err, attest.Sprintf("failed: %s ", test.domain))
			} else {
				attest.NotZero(t, err, attest.Sprintf("failed: %s ", test.domain))
			}
		}
	})
}
