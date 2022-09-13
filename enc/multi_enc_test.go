package enc

import (
	"sync"
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

	t.Run("encrypt/decrypt", func(t *testing.T) {
		t.Parallel()

		msgToEncryt := "hello world!"
		key1, key2 := getMultiSecretKeys()
		enc := NewMulti(key1, key2)

		encryptedMsg1, encryptedMsg2 := enc.encrypt(msgToEncryt)

		decryptedMsg, err := enc.decrypt(encryptedMsg1, encryptedMsg2)
		attest.Ok(t, err)

		attest.Equal(t, string(decryptedMsg), msgToEncryt)
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		msgToEncryt := "hello world!"

		run := func() {
			key1, key2 := getMultiSecretKeys()
			enc := NewMulti(key1, key2)

			encryptedMsg1, encryptedMsg2 := enc.encrypt(msgToEncryt)
			decryptedMsg, err := enc.decrypt(encryptedMsg1, encryptedMsg2)
			attest.Ok(t, err)
			attest.Equal(t, string(decryptedMsg), msgToEncryt)
		}

		wg := &sync.WaitGroup{}
		for rN := 0; rN <= 7; rN++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				run()
			}()
		}
		wg.Wait()
	})
}
