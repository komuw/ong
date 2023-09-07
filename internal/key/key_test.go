package key

import (
	"testing"

	"github.com/komuw/ong/internal/tst"
	"go.akshayshah.org/attest"
)

func TestCheckSecretKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		key   string
		check func(error)
	}{
		{
			name: "good key",
			key:  tst.SecretKey(),
			check: func(err error) {
				attest.Ok(t, err)
			},
		},
		{
			name: "uuid accepted",
			key:  "663acecd-af38-4e02-9529-1498bd7bd96e",
			check: func(err error) {
				attest.Ok(t, err)
			},
		},
		{
			name: "small secure key is ok",
			key:  "4$kplejewjdsnv",
			check: func(err error) {
				attest.Ok(t, err)
			},
		},
		{
			name: "bad key",
			key:  "super-h@rd-password",
			check: func(err error) {
				attest.Error(t, err)
			},
		},
		{
			name: "repeated key",
			key:  "4$aaaaaaaaaaaaa",
			check: func(err error) {
				attest.Error(t, err)
			},
		},
	}
	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := IsSecure(tt.key)
			tt.check(err)
		})
	}
}
