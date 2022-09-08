package middleware

import (
	"sync"
	"testing"

	"github.com/akshayjshah/attest"
	"golang.org/x/exp/slices"
)

func TestSecret(t *testing.T) {
	t.Parallel()

	t.Run("encrypt/decrypt", func(t *testing.T) {
		t.Parallel()

		msgToEncryt := "hello world!"
		key := getKey()

		encryptedMsg, err := encrypt(key, msgToEncryt)
		attest.Ok(t, err)

		decryptedMsg, err := decrypt(key, encryptedMsg)
		attest.Ok(t, err)

		attest.Equal(t, string(decryptedMsg), msgToEncryt)
	})

	t.Run("encrypt/decrypt base64", func(t *testing.T) {
		t.Parallel()

		msgToEncryt := "hello world!"
		key := getKey()

		encryptedMsg, err := encrypt(key, msgToEncryt)
		attest.Ok(t, err)

		token := encode(encryptedMsg)

		encryptedMsg2, err := decode(token)
		decryptedMsg, err := decrypt(key, encryptedMsg2)
		attest.Ok(t, err)

		attest.Equal(t, string(decryptedMsg), msgToEncryt)
	})

	t.Run("encrypt same msg is unique", func(t *testing.T) {
		t.Parallel()

		// This is a useful property especially in how we use it in csrf protection
		// against breachattack.

		msgToEncryt := "hello world!"
		key := getKey()

		encryptedMsg, err := encrypt(key, msgToEncryt)
		attest.Ok(t, err)

		var em []byte
		for i := 0; i < 4; i++ {
			em, err = encrypt(key, msgToEncryt)
			attest.Ok(t, err)
			if slices.Equal(encryptedMsg, em) {
				t.Fatal("slices should not be equal")
			}
		}

		decryptedMsg, err := decrypt(key, encryptedMsg)
		attest.Ok(t, err)
		attest.Equal(t, string(decryptedMsg), msgToEncryt)

		decryptedMsgForEm, err := decrypt(key, em)
		attest.Ok(t, err)
		attest.Equal(t, string(decryptedMsgForEm), msgToEncryt)
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		msgToEncryt := "hello world!"

		run := func() {
			key := getKey()
			encryptedMsg, err := encrypt(key, msgToEncryt)
			attest.Ok(t, err)
			decryptedMsg, err := decrypt(key, encryptedMsg)
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
