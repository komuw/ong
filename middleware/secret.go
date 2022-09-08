package middleware

import (
	cryptoRand "crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	mathRand "math/rand"
	"time"

	"golang.org/x/crypto/chacha20poly1305"
)

// Latacora recommends ChaCha20-Poly1305 for encryption.
// https://latacora.micro.blog/2018/04/03/cryptographic-right-answers.html
//
// The Go authors also recommend to use `crypto/cipher.NewGCM` or `XChaCha20-Poly1305`
// https://github.com/golang/crypto/blob/05595931fe9d3f8894ab063e1981d28e9873e2cb/tea/cipher.go#L13-L14
//
// XChaCha20-Poly1305, unlinke aes-gcm, has no message limit per key.
// It can safely encrypt an unlimited number of messages with the same key, without any limit to the size of a message.
// see: https://libsodium.gitbook.io/doc/secret-key_cryptography/aead/chacha20-poly1305/xchacha20-poly1305_construction

func rand(n1, n2 int) []byte {
	b := make([]byte, n1, n2)
	if _, err := cryptoRand.Read(b); err != nil {
		b = make([]byte, n1, n2)
		//
		// Since this func(rand()) is called in encrypt, Is it safe to use mathRand here?
		// According to agl(the one and only);
		// "The nonce itself does not have to be random, it can be a counter. But it absolutely must be unique"
		// "You can send the nonce in the clear before each message; so long as it's unique."
		// see: https://crypto.stackexchange.com/a/5818
		//
		mathRand.Seed(time.Now().UTC().UnixNano()) // this codepath is rarely executed so we can put all the code here instead of global var.
		_, _ = mathRand.Read(b)                    // docs say that it always returns a nil error.
	}
	return b
}

// TODO: this func should be called only once.
func getKey() []byte {
	// secretKey comes from the user.

	/*
		The draft RFC recommends[2] time=3, and memory=32*1024 is a sensible number.
		If using that amount of memory (32 MB) is not possible in some contexts then the time parameter can be increased to compensate.
		The number of threads can be adjusted to the number of available CPUs.
		salt should be random.
		- https://pkg.go.dev/golang.org/x/crypto/argon2#Key
	*/
	// time := uint32(3)
	// memory := uint32(32 * 1024) // 32MB
	// threads := uint8(4)
	// salt := rand(16, 16) // 16bytes are recommended
	// key := argon2.Key(
	// 	[]byte(secretKey),
	// 	salt,
	// 	time,
	// 	memory,
	// 	threads,
	// 	chacha20poly1305.KeySize,
	// )

	key := []byte("the key should 32bytes & random.")
	if len(key) != chacha20poly1305.KeySize {
		panic(fmt.Sprintf("key should have length of %d", chacha20poly1305.KeySize))
	}
	return key
}

func encrypt(key []byte, msg string) ([]byte, error) {
	msgToEncryt := []byte(msg)

	// xchacha20poly1305 takes a longer nonce, suitable to be generated randomly without risk of collisions.
	// It should be preferred when nonce uniqueness cannot be trivially ensured
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}

	// Select a random nonce, and leave capacity for the ciphertext.
	nonce := rand(aead.NonceSize(), aead.NonceSize()+len(msgToEncryt)+aead.Overhead())

	// Encrypt the message and append the ciphertext to the nonce.
	encryptedMsg := aead.Seal(nonce, nonce, msgToEncryt, nil)

	return encryptedMsg, nil
}

func decrypt(key, encryptedMsg []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}

	if len(encryptedMsg) < aead.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}

	// Split nonce and ciphertext.
	nonce, ciphertext := encryptedMsg[:aead.NonceSize()], encryptedMsg[aead.NonceSize():]

	// Decrypt the message and check it wasn't tampered with.
	decryptedMsg, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return decryptedMsg, nil
}

func encode(payload []byte) string {
	return base64.RawURLEncoding.EncodeToString(payload)
}

func decode(payload string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(payload)
}
