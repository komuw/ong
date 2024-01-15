package acme

import (
	"crypto/tls"
	"crypto/x509"
	"testing"
	"time"

	"go.akshayshah.org/attest"
)

// taken from  https://github.com/golang/crypto/blob/05595931fe9d3f8894ab063e1981d28e9873e2cb/acme/autocert/autocert_test.go#L672
func TestWildcardHostWhitelist(t *testing.T) {
	t.Parallel()

	t.Run("one domain", func(t *testing.T) {
		t.Parallel()

		policy := wildcardHostWhitelist("example.com")
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
			err := policy(test.host)
			if err != nil && test.allow {
				t.Errorf("%d: policy(%q): %v; want nil", i, test.host, err)
			}
			if err == nil && !test.allow {
				t.Errorf("%d: policy(%q): nil; want an error", i, test.host)
			}
		}
	})

	t.Run("sub-domain", func(t *testing.T) {
		t.Parallel()

		policy := wildcardHostWhitelist("api.example.com")
		tt := []struct {
			host  string
			allow bool
		}{
			{"api.example.com", true},
			{"api.EXAMPLE.com", true},
			{"API.EXAMPLE.COM", true},
			//
			{"example.com", false},
			{"one.example.com", false},
			{"one.abcd.example.com", false},
			//
			{"example.org", false},
			{"xn--9caa.com", false}, // éé.com
			{"two.example.org", false},
			{"three.example.net", false},
			{"dummy", false},
		}
		for i, test := range tt {
			err := policy(test.host)
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

		policy := wildcardHostWhitelist("éÉ.com")
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
			err := policy(test.host)
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

		policy := wildcardHostWhitelist("EXAMPLE.ORG")
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
			err := policy(test.host)
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

		policy := wildcardHostWhitelist("*.example.com")
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
			err := policy(test.host)
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

		policy := wildcardHostWhitelist("*.EXAMPLE.COM")
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
			err := policy(test.host)
			if err != nil && test.allow {
				t.Errorf("%d: policy(%q): %v; want nil", i, test.host, err)
			}
			if err == nil && !test.allow {
				t.Errorf("%d: policy(%q): nil; want an error", i, test.host)
			}
		}
	})
}

func TestCertIsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cert *tls.Certificate
		want bool
	}{
		{
			name: "nil cert",
			cert: nil,
			want: false,
		},
		{
			name: "certificate is too early",
			cert: &tls.Certificate{
				Leaf: &x509.Certificate{
					NotBefore: time.Now().UTC().Add(5 * 24 * time.Hour),      // In 5days time
					NotAfter:  time.Now().UTC().Add(3 * 30 * 24 * time.Hour), // 3months
				},
			},
			want: false,
		},
		{
			name: "certificate is okay",
			cert: &tls.Certificate{
				Leaf: &x509.Certificate{
					NotBefore: time.Now().UTC(),                              // Today
					NotAfter:  time.Now().UTC().Add(3 * 30 * 24 * time.Hour), // 3months
				},
			},
			want: true,
		},
		{
			name: "certificate is expired",
			cert: &tls.Certificate{
				Leaf: &x509.Certificate{
					NotBefore: time.Now().UTC(),                          // Today
					NotAfter:  time.Now().UTC().Add(-1 * 24 * time.Hour), // Yesterday
				},
			},
			want: false,
		},
		{
			name: "certificate is almost expired",
			cert: &tls.Certificate{
				Leaf: &x509.Certificate{
					NotBefore: time.Now().UTC(),                         // Today
					NotAfter:  time.Now().UTC().Add(2 * 24 * time.Hour), // 2days later.
				},
			},
			want: false,
		},
		{
			name: "expires in 7days",
			cert: &tls.Certificate{
				Leaf: &x509.Certificate{
					NotBefore: time.Now().UTC(),                         // Today
					NotAfter:  time.Now().UTC().Add(7 * 24 * time.Hour), // 7days later.
				},
			},
			want: true,
		},
		{
			// Let's encrypt backdates certificates by one hour to allow for clock skew.
			// See: https://community.letsencrypt.org/t/time-zone-considerations-needed-for-certificates/23130/2
			name: "certificate backdated by few hours",
			cert: &tls.Certificate{
				Leaf: &x509.Certificate{
					NotBefore: time.Now().UTC().Add(-3 * time.Hour),          // 3hrs ago
					NotAfter:  time.Now().UTC().Add(3 * 30 * 24 * time.Hour), // 3months
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := certIsValid(tt.cert)
			attest.Equal(t, got, tt.want)
		})
	}
}
