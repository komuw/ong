package key

import (
	"testing"

	"go.akshayshah.org/attest"
)

func TestCheckSecretKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		secretKey string
		check     func(error)
	}{
		{
			name:      "good key",
			secretKey: "super-h@rd-Pa$1word",
			check: func(err error) {
				attest.Ok(t, err)
			},
		},
		{
			name:      "uuid accepted",
			secretKey: "663acecd-af38-4e02-9529-1498bd7bd96e",
			check: func(err error) {
				attest.Ok(t, err)
			},
		},
		{
			name:      "bad key",
			secretKey: "super-h@rd-password",
			check: func(err error) {
				attest.Error(t, err)
			},
		},
	}
	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := IsSecure(tt.secretKey)
			tt.check(err)
		})
	}
}
