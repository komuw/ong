package enc

import (
	"testing"

	"github.com/akshayjshah/attest"
)

func getMultiSecretKeys() (string, string) {
	return "hello world", "its a new day."
}

func TestMultiEnc(t *testing.T) {
	t.Parallel()

	t.Run("new", func(t *testing.T) {
		t.Parallel()

		// okay key
		key1, key2 := getMultiSecretKeys()
		_ = NewMulti(key1, key2)

		// short keys
		attest.Panics(t, func() {
			_ = NewMulti("hi", key2)
		})
		attest.Panics(t, func() {
			_ = NewMulti(key1, "hi")
		})
	})
}
