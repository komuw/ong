package cry

import (
	"strings"
	"testing"

	"go.akshayshah.org/attest"
)

func TestHash(t *testing.T) {
	t.Parallel()

	t.Run("hash success", func(t *testing.T) {
		t.Parallel()

		password := "hey ok"
		hash := Hash(password)
		attest.NotZero(t, hash)
	})

	t.Run("eql success", func(t *testing.T) {
		t.Parallel()

		password := "hey ok"
		hash := Hash(password)
		attest.NotZero(t, hash)

		err := Eql(password, hash)
		attest.Ok(t, err)
	})

	t.Run("eql error", func(t *testing.T) {
		t.Parallel()

		password := "hey ok"
		hash := Hash(password)
		attest.NotZero(t, hash)

		hash = strings.ReplaceAll(hash, separator, "-")
		err := Eql(password, hash)
		attest.Error(t, err)
	})
}
