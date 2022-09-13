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
	salt := random(saltLen, saltLen)
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

// TODO: Encrypt & Decrypt should be private methods. Only the base64 methods should be public.
func (m *MultiEnc) Encrypt(plainTextMsg string) (encryptedMsg1 []byte, encryptedMsg2 []byte) {
	msgToEncryt := []byte(plainTextMsg)

	nonce := random(
		m.aead1.NonceSize(),
		m.aead1.NonceSize()+len(msgToEncryt)+m.aead1.Overhead(),
	)

	encrypted1 := m.aead1.Seal(nonce, nonce, msgToEncryt, nil)
	encrypted2 := m.aead2.Seal(nonce, nonce, msgToEncryt, nil)

	encrypted1 = append(m.salt, encrypted1...)
	encrypted2 = append(m.salt, encrypted2...)

	return encrypted1, encrypted2
}

func (m *MultiEnc) Decrypt(encryptedMsg1 []byte, encryptedMsg2 []byte) (decryptedMsg1 []byte, decryptedMsg2 []byte, err error) {
	if len(encryptedMsg1) < m.aead1.NonceSize() {
		return nil, nil, errors.New("ciphertext too short")
	}
	if len(encryptedMsg2) < m.aead2.NonceSize() {
		return nil, nil, errors.New("ciphertext too short")
	}

	// get salt
	salt1, encryptedMsg1 := encryptedMsg1[:saltLen], encryptedMsg1[saltLen:]
	salt2, encryptedMsg2 := encryptedMsg2[:saltLen], encryptedMsg2[saltLen:]

	{
		aead1 := m.aead1
		if !slices.Equal(salt1, m.salt) {
			// The encryptedMsg was encrypted using a different salt.
			// So, we need to get the derived key for that salt and use it for decryption.
			derivedKey1, errK := scrypt.Key(m.key1, salt1, N, r, p, chacha20poly1305.KeySize)
			if errK != nil {
				return nil, nil, errK
			}

			aead1, err = chacha20poly1305.NewX(derivedKey1)
			if err != nil {
				return nil, nil, err
			}
		}

		// Split nonce and ciphertext.
		nonce1, ciphertext1 := encryptedMsg1[:aead1.NonceSize()], encryptedMsg1[aead1.NonceSize():]

		// Decrypt the message and check it wasn't tampered with.
		decryptedMsg1, err = aead1.Open(nil, nonce1, ciphertext1, nil)
	}

	{
		aead2 := m.aead2
		if !slices.Equal(salt2, m.salt) {
			// The encryptedMsg was encrypted using a different salt.
			// So, we need to get the derived key for that salt and use it for decryption.
			derivedKey2, errK := scrypt.Key(m.key2, salt2, N, r, p, chacha20poly1305.KeySize)
			if errK != nil {
				return nil, nil, errK
			}

			aead2, err = chacha20poly1305.NewX(derivedKey2)
			if err != nil {
				return nil, nil, err
			}
		}

		// Split nonce and ciphertext.
		nonce2, ciphertext2 := encryptedMsg1[:aead2.NonceSize()], encryptedMsg1[aead2.NonceSize():]

		// Decrypt the message and check it wasn't tampered with.
		decryptedMsg2, err = aead2.Open(nil, nonce2, ciphertext2, nil)
	}

	return decryptedMsg1, decryptedMsg2, err
}
