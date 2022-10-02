// Package cry provides utilities for cryptography.
// This library has not been vetted and people are discouraged from using it.
// Instead use the crypto facilities in the Go standard library and/or golang.org/x/crypto
package cry

import (
	"crypto/cipher"
	cryptoRand "crypto/rand"
	"encoding/base64"
	"errors"
	mathRand "math/rand"
	"time"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/scrypt"
	"golang.org/x/exp/slices"
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

const (
	keyLen  = chacha20poly1305.KeySize
	saltLen = 8
	//
	// The values recommended as of year 2017 are:
	// n=32768, r=8 and p=1
	// https://pkg.go.dev/golang.org/x/crypto/scrypt#Key
	//
	n = 32768 // CPU/memory cost parameter.
	r = 8     // r and p must satisfy r * p < 2³⁰, else [scrypt.Key] returns an error.
	p = 1
)

// Enc is an AEAD cipher mode providing authenticated encryption with associated data, ie [cipher.AEAD]
//
// Use [New] to get a valid Enc.
type Enc struct {
	aead cipher.AEAD
	salt []byte
	key  []byte
}

// New returns a [cipher.AEAD]
//
// It panics on error.
//
// It uses [scrypt] to derive the final key that will be used for encryption.
func New(key string) Enc {
	// I think it is okay for New to panic instead of returning an error.
	// Since this is a crypto library, it is better to fail loudly than fail silently.
	//

	if len(key) < 4 {
		panic(errors.New("short key"))
	}

	// derive a key.
	password := []byte(key)
	salt := random(saltLen, saltLen) // should be random, 8 bytes is a good length.
	derivedKey, err := deriveKey(password, salt)
	if err != nil {
		panic(err)
	}

	/*
		Another option would be to use argon2.
		  import "golang.org/x/crypto/argon2"
		  salt := rand(16, 16) // 16bytes are recommended
		  key := argon2.Key( []byte("secretKey"), salt, 3, 32 * 1024, 4, keyLen)
	*/

	// xchacha20poly1305 takes a longer nonce, suitable to be generated randomly without risk of collisions.
	// It should be preferred when nonce uniqueness cannot be trivially ensured
	aead, err := chacha20poly1305.NewX(derivedKey)
	if err != nil {
		panic(err)
	}

	return Enc{
		aead: aead,
		salt: salt,
		key:  password,
	}
}

// Encrypt, encrypts and authenticates(tamper-proofs) the plainTextMsg using XChaCha20-Poly1305 and returns encrypted bytes.
func (e Enc) Encrypt(plainTextMsg string) (encryptedMsg []byte) {
	msgToEncrypt := []byte(plainTextMsg)

	// Select a random nonce, and leave capacity for the ciphertext.
	nonce := random(
		e.aead.NonceSize(),
		e.aead.NonceSize()+len(msgToEncrypt)+e.aead.Overhead(),
	)

	// Encrypt the message and append the ciphertext to the nonce.
	encrypted := e.aead.Seal(nonce, nonce, msgToEncrypt, nil)

	encrypted = append(
		// "you can send the nonce in the clear before each message; so long as it's unique." - agl
		// see: https://crypto.stackexchange.com/a/5818
		//
		// "salt does not need to be secret."
		// see: https://crypto.stackexchange.com/a/99502
		e.salt,
		encrypted...,
	)

	return encrypted
}

// Decrypt authenticates and un-encrypts the encryptedMsg using XChaCha20-Poly1305 and returns decrypted bytes.
func (e Enc) Decrypt(encryptedMsg []byte) (decryptedMsg []byte, err error) {
	if len(encryptedMsg) < e.aead.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}

	// get salt
	salt, encryptedMsg := encryptedMsg[:saltLen], encryptedMsg[saltLen:]

	aead := e.aead
	if !slices.Equal(salt, e.salt) {
		// The encryptedMsg was encrypted using a different salt.
		// So, we need to get the derived key for that salt and use it for decryption.
		derivedKey, errK := scrypt.Key(e.key, salt, n, r, p, keyLen)
		if errK != nil {
			return nil, errK
		}

		aead, err = chacha20poly1305.NewX(derivedKey)
		if err != nil {
			return nil, err
		}
	}

	// Split nonce and ciphertext.
	nonce, ciphertext := encryptedMsg[:aead.NonceSize()], encryptedMsg[aead.NonceSize():]

	// Decrypt the message and check it wasn't tampered with.
	return aead.Open(nil, nonce, ciphertext, nil)
}

// EncryptEncode is like [Enc.Encrypt] except that it returns a string that is encoded using [base64.RawURLEncoding]
func (e Enc) EncryptEncode(plainTextMsg string) (encryptedEncodedMsg string) {
	return base64.RawURLEncoding.EncodeToString(e.Encrypt(plainTextMsg))
}

// DecryptDecode takes an encryptedEncodedMsg that was generated using [Enc.EncryptEncode] and returns the original un-encrypted string.
func (e Enc) DecryptDecode(encryptedEncodedMsg string) (plainTextMsg string, err error) {
	encryptedMsg, err := base64.RawURLEncoding.DecodeString(encryptedEncodedMsg)
	if err != nil {
		return "", err
	}

	decrypted, err := e.Decrypt(encryptedMsg)
	if err != nil {
		return "", err
	}

	return string(decrypted), nil
}

func random(n1, n2 int) []byte {
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
