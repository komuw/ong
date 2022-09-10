// Package enc provides utilities to carry out encryption and decryption.
// This library has not been vetted and people are discouraged from using it.
// Instead use the crypto facilities in the Go standard library and/or golang.org/x/crypto
package enc

import (
	"crypto/cipher"
	cryptoRand "crypto/rand"
	"encoding/base64"
	"errors"
	mathRand "math/rand"
	"time"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/scrypt"
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

// Enc is an AEAD cipher mode providing authenticated encryption with associated data.
// Use [New] to get a valid Enc.
// see [cipher.AEAD]
type Enc struct {
	a cipher.AEAD
}

// New returns a [cipher.AEAD]
// The key should be random.
// New panics if key is too small in length or if it is full of similar bytes.
func New(key []byte) *Enc {
	// I think it is okay for New to panic instead of returning an error.
	// Since this is a crypto library, it is better to fail loudly than fail silently.
	//

	if len(key) < 8 {
		panic(errors.New("short key"))
	}

	isRandom := false
	firstChar := key[0]
	for _, v := range key[1:] {
		// if all the elements in the slice are equal, then the key is not random.
		if v != firstChar {
			isRandom = true
			break
		}
	}

	if !isRandom {
		panic(errors.New("the key is not random"))
	}

	// derive a key.
	salt := random(8, 8) // should be random, 8 bytes is a good length.
	N := 32768           // CPU/memory cost parameter.
	r := 8               // r and p must satisfy r * p < 2³⁰, else [scrypt.Key] returns an error.
	p := 1
	derivedKey, err := scrypt.Key(
		key,
		salt,
		// The values recommended as of year 2017 are:
		// N=32768, r=8 and p=1
		// https://pkg.go.dev/golang.org/x/crypto/scrypt#Key
		N,
		r,
		p,
		chacha20poly1305.KeySize,
	)
	if err != nil {
		panic(err)
	}

	// xchacha20poly1305 takes a longer nonce, suitable to be generated randomly without risk of collisions.
	// It should be preferred when nonce uniqueness cannot be trivially ensured
	aead, err := chacha20poly1305.NewX(derivedKey)
	if err != nil {
		panic(err)
	}

	return &Enc{aead}
}

// Encrypt encrypts the plainTextMsg using XChaCha20-Poly1305 and returns encrypted bytes.
func (e *Enc) Encrypt(plainTextMsg string) (encryptedMsg []byte) {
	msgToEncryt := []byte(plainTextMsg)

	// Select a random nonce, and leave capacity for the ciphertext.
	nonce := random(e.a.NonceSize(), e.a.NonceSize()+len(msgToEncryt)+e.a.Overhead())

	// Encrypt the message and append the ciphertext to the nonce.
	return e.a.Seal(nonce, nonce, msgToEncryt, nil)
}

// Decrypt un-encrypts the encryptedMsg using XChaCha20-Poly1305 and returns decryted bytes.
func (e *Enc) Decrypt(encryptedMsg []byte) (decryptedMsg []byte, err error) {
	if len(encryptedMsg) < e.a.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}

	// Split nonce and ciphertext.
	nonce, ciphertext := encryptedMsg[:e.a.NonceSize()], encryptedMsg[e.a.NonceSize():]

	// Decrypt the message and check it wasn't tampered with.
	return e.a.Open(nil, nonce, ciphertext, nil)
}

// EncryptEncode is like [Encrypt] except that it returns a string that is encoded using [base64.RawURLEncoding]
func (e *Enc) EncryptEncode(plainTextMsg string) (encryptedEncodedMsg string) {
	return base64.RawURLEncoding.EncodeToString(e.Encrypt(plainTextMsg))
}

// DecryptDecode takes an encryptedEncodedMsg that was generated using [EncryptEncode] and returns the original un-encrypted string.
func (e *Enc) DecryptDecode(encryptedEncodedMsg string) (plainTextMsg string, err error) {
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
