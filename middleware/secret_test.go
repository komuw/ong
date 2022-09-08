package middleware

import (
	"sync"
	"testing"

	"github.com/akshayjshah/attest"
)

func TestSecret(t *testing.T) {
	t.Parallel()

	t.Run("encrypt/decrypt", func(t *testing.T) {
		t.Parallel()

		secretKey := "hey"
		msgToEncryt := "hello world!"
		key := getKey(secretKey)

		encryptedMsg, err := encrypt(key, msgToEncryt)
		attest.Ok(t, err)

		decryptedMsg, err := decrypt(key, encryptedMsg)
		attest.Ok(t, err)

		attest.Equal(t, string(decryptedMsg), msgToEncryt)
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		secretKey := "hey"
		msgToEncryt := "hello world!"

		run := func() {
			key := getKey(secretKey)
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
