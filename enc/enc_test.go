package enc

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
			[]byte("secretKey"),
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

	t.Run("new", func(t *testing.T) {
		t.Parallel()

		// okay key
		key := getSecretKey()
		_ = New(key)

		// short key
		attest.Panics(t, func() {
			_ = New([]byte{1, 3, 8})
		})

		// non-random key
		key = getSecretKey()
		for j := range key {
			key[j] = 'a'
		}
		attest.Panics(t, func() {
			_ = New(key)
		})
	})

	t.Run("encrypt/decrypt", func(t *testing.T) {
		t.Parallel()

		msgToEncryt := "hello world!"
		key := getSecretKey()
		enc := New(key)

		encryptedMsg := enc.Encrypt(msgToEncryt)

		decryptedMsg, err := enc.Decrypt(encryptedMsg)
		attest.Ok(t, err)

		attest.Equal(t, string(decryptedMsg), msgToEncryt)
	})

	t.Run("encrypt/decrypt base64", func(t *testing.T) {
		t.Parallel()

		msgToEncryt := "hello world!"
		key := getSecretKey()
		enc := New(key)

		token := enc.EncryptEncode(msgToEncryt)

		decryptedMsg, err := enc.DecryptDecode(token)
		attest.Ok(t, err)
		attest.Equal(t, string(decryptedMsg), msgToEncryt)
	})

	t.Run("encrypt same msg is unique", func(t *testing.T) {
		t.Parallel()

		// This is a useful property especially in how we use it in csrf protection
		// against breachattack.

		msgToEncryt := "hello world!"
		key := getSecretKey()
		enc := New(key)

		encryptedMsg := enc.Encrypt(msgToEncryt)

		var em []byte
		for i := 0; i < 4; i++ {
			em = enc.Encrypt(msgToEncryt)
			if slices.Equal(encryptedMsg, em) {
				t.Fatal("slices should not be equal")
			}
		}

		decryptedMsg, err := enc.Decrypt(encryptedMsg)
		attest.Ok(t, err)
		attest.Equal(t, string(decryptedMsg), msgToEncryt)

		decryptedMsgForEm, err := enc.Decrypt(em)
		attest.Ok(t, err)
		attest.Equal(t, string(decryptedMsgForEm), msgToEncryt)
	})

	t.Run("same input key will always be able to encrypt and decrypt", func(t *testing.T) {
		t.Parallel()

		msgToEncryt := "hello world!"
		key := getSecretKey()

		{
			enc1 := New(key)

			encryptedMsg := enc1.Encrypt(msgToEncryt)
			decryptedMsg, err := enc1.Decrypt(encryptedMsg)
			attest.Ok(t, err)
			attest.Equal(t, string(decryptedMsg), msgToEncryt)
		}

		{
			enc2 := New(key)

			encryptedMsg := enc2.Encrypt(msgToEncryt)
			decryptedMsg, err := enc2.Decrypt(encryptedMsg)
			attest.Ok(t, err)
			attest.Equal(t, string(decryptedMsg), msgToEncryt)
		}
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		msgToEncryt := "hello world!"

		run := func() {
			key := getSecretKey()
			enc := New(key)

			encryptedMsg := enc.Encrypt(msgToEncryt)
			decryptedMsg, err := enc.Decrypt(encryptedMsg)
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
