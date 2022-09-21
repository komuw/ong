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

		token := enc.EncryptEncode(msgToEncryt)

		decryptedMsg, err := enc.DecryptDecode(token)
		attest.Ok(t, err)
		attest.Equal(t, decryptedMsg, msgToEncryt)
	})

	t.Run("same key again", func(t *testing.T) {
		t.Parallel()

		msgToEncryt := "hello world!"
		key1 := "okay what are you"
		key2 := "kill it with fire"

		tEnc := NewMulti(key1, key2)

		token := tEnc.EncryptEncode(msgToEncryt)

		decryptedMsg, err := tEnc.DecryptDecode(token)
		attest.Ok(t, err)
		attest.Equal(t, decryptedMsg, msgToEncryt)

		tEncX := NewMulti(key1, key2)
		decryptedMsg2, err := tEncX.DecryptDecode(token)
		attest.Ok(t, err)
		attest.Equal(t, decryptedMsg2, msgToEncryt)
	})

	t.Run("key rotation", func(t *testing.T) {
		t.Parallel()

		msgToEncryt := "hello world!"
		key1 := "okay what are you"
		key2 := "kill it with fire"

		tEnc := NewMulti(key1, key2)

		token := tEnc.EncryptEncode(msgToEncryt)

		decryptedMsg, err := tEnc.DecryptDecode(token)
		attest.Ok(t, err)
		attest.Equal(t, decryptedMsg, msgToEncryt)

		rotatedKey2 := "brand new key2"
		tEncX := NewMulti(key1, rotatedKey2)
		decryptedMsg2, err := tEncX.DecryptDecode(token)
		attest.Ok(t, err)
		attest.Equal(t, decryptedMsg2, msgToEncryt)
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		msgToEncryt := "hello world!"

		run := func() {
			key1, key2 := getMultiSecretKeys()
			enc := NewMulti(key1, key2)

			token := enc.EncryptEncode(msgToEncryt)
			decryptedMsg, err := enc.DecryptDecode(token)
			attest.Ok(t, err)
			attest.Equal(t, decryptedMsg, msgToEncryt)
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
