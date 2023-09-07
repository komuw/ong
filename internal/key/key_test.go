package key

import (
	"testing"

	"github.com/komuw/ong/id"
	"github.com/komuw/ong/internal/tst"
	"go.akshayshah.org/attest"
)

func TestCheckSecretKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		key          string
		shouldSucced bool
	}{
		{
			name:         "good key",
			key:          tst.SecretKey(),
			shouldSucced: true,
		},
		{
			name:         "uuid accepted",
			key:          "663acecd-af38-4e02-9529-1498bd7bd96e",
			shouldSucced: true,
		},
		{
			name:         "ong/id.New()",
			key:          id.New(),
			shouldSucced: true,
		},
		{
			name:         "small secure key is ok",
			key:          "4$aBCdEfGhIjKlMn",
			shouldSucced: true,
		},
		{
			name:         "bad key",
			key:          "super-h@rd-password",
			shouldSucced: false,
		},
		{
			name:         "repeated key",
			key:          "4$7kBaaaaaaaaaaaaa",
			shouldSucced: false,
		},
	}
	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := IsSecure(tt.key)
			if tt.shouldSucced {
				attest.Ok(t, err)
			} else {
				attest.Error(t, err)
			}
		})
	}
}
