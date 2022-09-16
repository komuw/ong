package enc

import (
	"encoding/base64"
	"errors"
	"strings"
)

const (
	// This should be a character that is not part of either;
	//   - base64.encodeStd
	//   - base64.encodeURL
	separator = ":"
)

type MultiEnc struct {
	enc1 Enc
	enc2 Enc
}

// TODO: mention that you can only rotate one key at a time.
func NewMulti(key1, key2 string) MultiEnc {
	enc1 := New(key1)
	enc2 := New(key2)
	return MultiEnc{
		enc1: enc1,
		enc2: enc2,
	}
}

func (m MultiEnc) EncryptEncode(plainTextMsg string) (encryptedEncodedMsg string) {
	encryptedMsg1, encryptedMsg2 := m.enc1.Encrypt(plainTextMsg), m.enc2.Encrypt(plainTextMsg)
	encoded1 := base64.RawURLEncoding.EncodeToString(encryptedMsg1)
	encoded2 := base64.RawURLEncoding.EncodeToString(encryptedMsg2)
	return encoded1 + separator + encoded2
}

func (m MultiEnc) DecryptDecode(encryptedEncodedMsg string) (plainTextMsg string, err error) {
	encoded := strings.Split(encryptedEncodedMsg, separator)
	if len(encoded) != 2 {
		return "", errors.New("message was encoded incorrectly") // TODO: make these errors, constants.
	}

	resEncryptedMsg1, err1 := base64.RawURLEncoding.DecodeString(encoded[0])
	if err1 != nil {
		err = err1
	}
	resEncryptedMsg2, err2 := base64.RawURLEncoding.DecodeString(encoded[1])
	if err2 != nil {
		err = err2
	}

	if (err1 != nil) && (err2 != nil) {
		// This method should only fail if BOTH message decoding/decrypting also fail.
		// One failure should not cause us to fail.
		return "", err
	}

	decryptedMsg1, err1 := m.enc1.Decrypt(resEncryptedMsg1)
	if err1 != nil {
		err = err1
	}
	decryptedMsg2, err2 := m.enc2.Decrypt(resEncryptedMsg2)
	if err2 != nil {
		err = err2
	}

	if (err1 != nil) && (err2 != nil) {
		// This method should only fail if BOTH message decoding/decrypting also fail.
		// One failure should not cause us to fail.
		return "", err
	}

	if len(decryptedMsg1) > 0 {
		return string(decryptedMsg1), nil
	}

	return string(decryptedMsg2), nil
}
