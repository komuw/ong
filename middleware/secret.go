package middleware

import (
	"crypto/cipher"
	cryptoRand "crypto/rand"
	"encoding/base64"
	"errors"
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
// see:
//   - https://gist.github.com/komuw/4d44a25e1b6786100ffe0308106e80f2
//   - https://libsodium.gitbook.io/doc/secret-key_cryptography/aead/chacha20-poly1305/xchacha20-poly1305_construction
//   - https://pycryptodome.readthedocs.io/en/latest/src/cipher/chacha20_poly1305.html
//
// This file uses [chacha20poly1305.NewX] which is XChaCha20-Poly1305.
//

const nulByte = '\x00'

// enc is an AEAD cipher mode providing authenticated encryption with associated data.
// see [cipher.AEAD]
type enc struct {
	cipher.AEAD
}

// NewEnc returns an [cipher.AEAD]
// The key should be random and 32 bytes in length.
func NewEnc(key []byte) (*enc, error) {
	isRandom := false
	// if all the elements in the slice are nul bytes, then the key is not random.
	for _, v := range key {
		if v != nulByte {
			isRandom = true
			break
		}
	}

	if !isRandom {
		return nil, errors.New("the secretKey is not random")
	}

	// xchacha20poly1305 takes a longer nonce, suitable to be generated randomly without risk of collisions.
	// It should be preferred when nonce uniqueness cannot be trivially ensured
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}

	return &enc{aead}, nil
}

// Encrypt encrypts the msg using XChaCha20-Poly1305
func (e *enc) Encrypt(msg string) []byte {
	msgToEncryt := []byte(msg)

	// Select a random nonce, and leave capacity for the ciphertext.
	nonce := rand(e.NonceSize(), e.NonceSize()+len(msgToEncryt)+e.Overhead())

	// Encrypt the message and append the ciphertext to the nonce.
	encryptedMsg := e.Seal(nonce, nonce, msgToEncryt, nil)

	return encryptedMsg
}

// Decrypt decrypts the encryptedMsg using XChaCha20-Poly1305
func (e *enc) Decrypt(encryptedMsg []byte) ([]byte, error) {
	if len(encryptedMsg) < e.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}

	// Split nonce and ciphertext.
	nonce, ciphertext := encryptedMsg[:e.NonceSize()], encryptedMsg[e.NonceSize():]

	// Decrypt the message and check it wasn't tampered with.
	decryptedMsg, err := e.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return decryptedMsg, nil
}

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

func encode(payload []byte) string {
	return base64.RawURLEncoding.EncodeToString(payload)
}

func decode(payload string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(payload)
}
