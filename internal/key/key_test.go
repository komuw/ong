package key

import (
	"bufio"
	"os"
	"testing"

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
			key:          "xC8R4RFWqtXE5DEf",
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

func TestCommon10kPasswords(t *testing.T) {
	t.Parallel()

	f, err := os.Open(
		// From https://github.com/danielmiessler/SecLists/blob/2023.3/Passwords/Common-Credentials/10-million-password-list-top-10000.txt
		// Although we remove the password `PolniyPizdec0211` from the list.
		"testdata/10k-most-common-passwords.txt",
	)
	attest.Ok(t, err)
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	// optionally, resize scanner's capacity for lines over 64K, see next example
	for scanner.Scan() {
		count = count + 1
		key := scanner.Text()
		err := IsSecure(key)
		attest.Error(t, err, attest.Sprintf("key(`%s`), count=%d from common password list.", key, count))
	}

	errS := scanner.Err()
	attest.Ok(t, errS)
	attest.True(t, count > 9_000)
}
