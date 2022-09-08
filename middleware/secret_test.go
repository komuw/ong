package middleware

import (
	"fmt"
	"sync"
	"testing"

	"github.com/akshayjshah/attest"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/exp/slices"
)

func getSecretKey() []byte {
	/*
		The draft RFC recommends[2] time=3, and memory=32*1024 is a sensible number.
		If using that amount of memory (32 MB) is not possible in some contexts then the time parameter can be increased to compensate.
		The number of threads can be adjusted to the number of available CPUs.
		salt should be random.
		- https://pkg.go.dev/golang.org/x/crypto/argon2#Key
	*/

	/*
		key should be randomly generated or derived from a function like Argon2.
			import "golang.org/x/crypto/argon2"

		time := uint32(3)
		memory := uint32(32 * 1024) // 32MB
		threads := uint8(4)
		salt := rand(16, 16) // 16bytes are recommended
		key := argon2.Key(
			[]byte(secretKey),
			salt,
			time,
			memory,
			threads,
			chacha20poly1305.KeySize,
		)
	*/

	key := []byte("key should be 32bytes and random")
	if len(key) != chacha20poly1305.KeySize {
		panic(fmt.Sprintf("key should have length of %d", chacha20poly1305.KeySize))
	}

	return key
}

func TestSecret(t *testing.T) {
	t.Parallel()

	t.Run("encrypt/decrypt", func(t *testing.T) {
		t.Parallel()

		msgToEncryt := "hello world!"
		key := getSecretKey()

		encryptedMsg, err := encrypt(key, msgToEncryt)
		attest.Ok(t, err)

		decryptedMsg, err := decrypt(key, encryptedMsg)
		attest.Ok(t, err)

		attest.Equal(t, string(decryptedMsg), msgToEncryt)
	})

	t.Run("encrypt/decrypt base64", func(t *testing.T) {
		t.Parallel()

		msgToEncryt := "hello world!"
		key := getSecretKey()

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
		key := getSecretKey()

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
			key := getSecretKey()
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
