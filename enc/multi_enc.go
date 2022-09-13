package enc

import (
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"strings"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/scrypt"
	"golang.org/x/exp/slices"
)

type msgNum string

const (
	one msgNum = "one"
	two msgNum = "two"
	// This should be a character that is not part of either;
	//   - base64.encodeStd
	//   - base64.encodeURL
	separator = ":"
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

func (m *MultiEnc) encrypt(plainTextMsg string) (encryptedMsg1, encryptedMsg2 []byte) {
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

func (m *MultiEnc) decrypt(encryptedMsg1, encryptedMsg2 []byte) (decryptedMsg []byte, err error) {
	//
	// In this method, we only fail if we are unable to use either of the keys.
	// If one fails and the other succeeds, then this method also succeeds.
	//

	decryptedMsg1, err1 := m.decryptMulti(encryptedMsg1, one)
	decryptedMsg2, err2 := m.decryptMulti(encryptedMsg2, two)

	if err1 != nil {
		err = err1
		decryptedMsg = decryptedMsg2
	}
	if err2 != nil {
		err = err2
		decryptedMsg = decryptedMsg1
	}

	if (err1 != nil) && (err2 != nil) {
		return nil, err
	}

	return decryptedMsg, nil
}

func (m *MultiEnc) decryptMulti(encryptedMsg []byte, mn msgNum) (decryptedMsg []byte, err error) {
	if !slices.Contains([]msgNum{one, two}, mn) {
		return nil, errors.New("msgNumber is not known")
	}

	var aead cipher.AEAD
	var key []byte
	if mn == one {
		aead = m.aead1
		key = m.key1
	} else {
		aead = m.aead2
		key = m.key2
	}

	if len(encryptedMsg) < aead.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}

	salt, encryptedMsg := encryptedMsg[:saltLen], encryptedMsg[saltLen:]
	if !slices.Equal(salt, m.salt) {
		derivedKey, errK := scrypt.Key(key, salt, N, r, p, chacha20poly1305.KeySize)
		if errK != nil {
			return nil, errK
		}

		aead, err = chacha20poly1305.NewX(derivedKey)
		if err != nil {
			return nil, err
		}
	}

	nonce, ciphertext := encryptedMsg[:aead.NonceSize()], encryptedMsg[aead.NonceSize():]
	return aead.Open(nil, nonce, ciphertext, nil)
}

func (m *MultiEnc) EncryptEncode(plainTextMsg string) (encryptedEncodedMsg string) {
	encryptedMsg1, encryptedMsg2 := m.encrypt(plainTextMsg)
	encoded1 := base64.RawURLEncoding.EncodeToString(encryptedMsg1)
	encoded2 := base64.RawURLEncoding.EncodeToString(encryptedMsg2)
	return encoded1 + separator + encoded2
}

func (m *MultiEnc) DecryptDecode(encryptedEncodedMsg string) (plainTextMsg string, err error) {
	encoded := strings.Split(encryptedEncodedMsg, separator)
	if len(encoded) != 2 {
		return "", errors.New("message was encoded incorrectly")
	}

	encryptedMsg1, err := base64.RawURLEncoding.DecodeString(encoded[0])
	if err != nil {
		return "", err
	}
	encryptedMsg2, err := base64.RawURLEncoding.DecodeString(encoded[1])
	if err != nil {
		return "", err
	}

	decrypted, err := m.decrypt(encryptedMsg1, encryptedMsg2)
	if err != nil {
		return "", err
	}

	return string(decrypted), nil
}
