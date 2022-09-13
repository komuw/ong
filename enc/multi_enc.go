package enc

import (
	"crypto/cipher"
	"errors"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/scrypt"
	"golang.org/x/exp/slices"
)

type MultiEnc struct {
	aead1 cipher.AEAD
	aead2 cipher.AEAD
	salt  []byte
	key1  []byte
	key2  []byte
}

func NewMulti(key1, key2 string) *MultiEnc {
	// I think it is okay for New to panic instead of returning an error.
	// Since this is a crypto library, it is better to fail loudly than fail silently.
	//

	if len(key1) < 4 {
		panic(errors.New("short key")) // TODO: make these errors, constants.
	}
	if len(key2) < 4 {
		panic(errors.New("short key"))
	}

	// derive a key.
	salt := random(saltLen, saltLen) // should be random, 8 bytes is a good length.
	password1 := []byte(key1)
	password2 := []byte(key2)
	derivedKey1, err := scrypt.Key(password1, salt, N, r, p, chacha20poly1305.KeySize)
	if err != nil {
		panic(err)
	}
	derivedKey2, err := scrypt.Key(password2, salt, N, r, p, chacha20poly1305.KeySize)
	if err != nil {
		panic(err)
	}

	/*
		Another option would be to use argon2.
		  import "golang.org/x/crypto/argon2"
		  salt := rand(16, 16) // 16bytes are recommended
		  key := argon2.Key( []byte("secretKey"), salt, 3, 32 * 1024, 4, chacha20poly1305.KeySize)
	*/

	// xchacha20poly1305 takes a longer nonce, suitable to be generated randomly without risk of collisions.
	// It should be preferred when nonce uniqueness cannot be trivially ensured
	aead1, err := chacha20poly1305.NewX(derivedKey1)
	if err != nil {
		panic(err)
	}
	aead2, err := chacha20poly1305.NewX(derivedKey2)
	if err != nil {
		panic(err)
	}

	return &MultiEnc{
		aead1: aead1,
		aead2: aead2,
		salt:  salt,
		key1:  password1,
		key2:  password2,
	}
}

func (m *MultiEnc) Encrypt(plainTextMsg string) (encryptedMsg []byte) {
	msgToEncryt := []byte(plainTextMsg)

	// Select a random nonce, and leave capacity for the ciphertext.
	nonce := random(
		m.aead1.NonceSize(),
		m.aead1.NonceSize()+len(msgToEncryt)+m.aead1.Overhead(),
	)

	var encrypted []byte
	encrypted1 := m.aead1.Seal(nonce, nonce, msgToEncryt, nil)
	encrypted2 := m.aead2.Seal(nonce, nonce, msgToEncryt, nil)

	encrypted = append(m.salt, encrypted1...)
	encrypted = append(encrypted, encrypted2...)
	return encrypted
}

func (m *MultiEnc) Decrypt(encryptedMsg []byte) (decryptedMsg []byte, err error) {
	if len(encryptedMsg) < (2 * m.aead1.NonceSize()) {
		return nil, errors.New("ciphertext too short")
	}

	// get salt
	salt, encryptedMsg := encryptedMsg[:saltLen], encryptedMsg[saltLen:]

	aead := m.aead
	if !slices.Equal(salt, m.salt) {
		// The encryptedMsg was encrypted using a different salt.
		// So, we need to get the derived key for that salt and use it for decryption.
		derivedKey, errK := scrypt.Key(m.key, salt, N, r, p, chacha20poly1305.KeySize)
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
