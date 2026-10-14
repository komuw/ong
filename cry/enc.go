// Package cry provides utilities for cryptography.
// This library has not been vetted and people are discouraged from using it.
// Instead, use the cryptography facilities in the Go standard library and/or golang.org/x/crypto
package cry

import (
	"crypto/cipher"
	cryptoRand "crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"

	"golang.org/x/crypto/chacha20poly1305"
)

// Latacora recommends ChaCha20-Poly1305 for encryption.
// https://www.latacora.com/blog/2024/07/29/crypto-right-answers-pq/
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
)

// Enc is an AEAD cipher mode providing authenticated encryption with associated data, ie [cipher.AEAD]
//
// Use [New] to get a valid Enc.
type Enc struct {
	aead cipher.AEAD
	salt []byte
}

// String implements [fmt.Stringer]
func (e Enc) String() string {
	if e.aead == nil {
		return "Enc{key:<EMPTY>}"
	}
	return "Enc{key:<REDACTED>}"
}

// GoString implements [fmt.GoStringer]
func (e Enc) GoString() string {
	return e.String()
}

// New returns a [cipher.AEAD]
//
// It panics on error.
//
// It uses argon2id to derive the final key that will be used for encryption.
func New(secretKey string) Enc {
	// I think it is okay for New to panic instead of returning an error.
	// Since this is a crypto library, it is better to fail loudly than fail silently.
	//

	if err := IsSecure(secretKey); err != nil {
		panic(err)
	}

	// derive a key.
	password := []byte(secretKey)
	salt := saltFor(password)
	derivedKey := deriveKey(password, salt)

	/*
		Another option would be to use scrypt.
		  import "golang.org/x/crypto/scrypt"
		  key := scrypt.Key("password", salt, 32768, 8, 1, keyLen)
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
	}
}

// saltFor returns the salt that is used to derive the encryption key from secretKey.
//
// It is the same in all processes that use the same secretKey. So a ciphertext from another process(eg after a restart,
// or from another server) can be decrypted without a new argon2 key derivation. And a ciphertext with any other salt can be
// rejected before argon2 is called; argon2 uses 64MB of memory and about 35ms, and clients can choose the salt(eg in a cookie).
//
// It does not need to be random. [IsSecure] makes sure that secretKey has enough entropy, so tables that are computed in advance do not help.
// The salt is also not secret; it is sent in clear text with each ciphertext.
func saltFor(secretKey []byte) []byte {
	h := sha256.New()
	h.Write([]byte("ong/cry: salt for encryption key\x00")) // domain separation.
	h.Write(secretKey)
	// The capacity is limited to saltLen, so that `append(e.salt, ...)` in [Enc.Encrypt] always makes a new slice.
	return h.Sum(nil)[:saltLen:saltLen]
}

// Encrypt, encrypts and authenticates(tamper-proofs) the plainTextMsg using XChaCha20-Poly1305 and returns encrypted bytes.
func (e Enc) Encrypt(plainTextMsg string) (encryptedMsg []byte) {
	msgToEncrypt := []byte(plainTextMsg)

	// Select a random nonce.
	// https://github.com/golang/crypto/blob/v0.26.0/chacha20poly1305/chacha20poly1305_test.go#L222
	nonce := random(
		chacha20poly1305.NonceSizeX,
		chacha20poly1305.NonceSizeX+len(msgToEncrypt)+chacha20poly1305.Overhead,
	)

	// Encrypt the message and append the ciphertext to the nonce.
	//
	// version as additionalData ensures that encryption/decryption will fail if using different versions of `ong/cry`
	// another option would be to prepend the version similar to salt.
	encrypted := e.aead.Seal(nonce, nonce, msgToEncrypt, []byte{version})

	// Append the salt & nonce to encrypted msg.
	// |salt|nonce|encryptedMsg|
	encrypted = append(
		// "you can send the nonce in the clear before each message; so long as it's unique. it can even be a counter." - agl
		// see: https://crypto.stackexchange.com/a/5818
		//
		// there are problems if you use a counter as nonce - https://blog.trailofbits.com/2024/09/13/friends-dont-let-friends-reuse-nonces/
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
	if len(encryptedMsg) < saltLen+chacha20poly1305.NonceSizeX {
		return nil, errors.New("ong/cry: ciphertext too short")
	}

	// get constituent parts
	salt, nonce, ciphertext := encryptedMsg[:saltLen], encryptedMsg[saltLen:saltLen+chacha20poly1305.NonceSizeX], encryptedMsg[saltLen+chacha20poly1305.NonceSizeX:]

	if subtle.ConstantTimeCompare(salt, e.salt) != 1 {
		// It was not encrypted with this secretKey, or it was encrypted by an older version of ong/cry that used random salts.
		// We do not derive a key for this salt, see [saltFor].
		return nil, errors.New("ong/cry: unknown salt")
	}

	// Decrypt the message and check it wasn't tampered with.
	return e.aead.Open(nil, nonce, ciphertext, []byte{version})
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
	_, _ = cryptoRand.Read(b) // it panics internally on error
	return b
}
