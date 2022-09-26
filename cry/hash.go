package cry

import (
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/scrypt"
)

// Most of the code here is insipired by(or taken from):
//   (a) https://github.com/elithrar/simple-scrypt whose license(MIT) can be found here: https://github.com/elithrar/simple-scrypt/blob/v1.3.0/LICENSE

const (
	// this should be increased every time the parameters passed to [scrypt.Key] are changed.
	version   = 1
	separator = "$"
)

func deriveKey(password, salt []byte) (derivedKey []byte, err error) {
	derivedKey, err = scrypt.Key(password, salt, n, r, p, keyLen)
	if err != nil {
		return nil, err
	}

	return derivedKey, nil
}

// Hash returns the scrypt hash of the password.
// It is safe to persist the result in your database instead of storing the actual password.
func Hash(password string) (string, error) {
	salt := random(saltLen, saltLen)
	derivedKey, err := deriveKey([]byte(password), salt)
	if err != nil {
		return "", err
	}

	// Add version, salt to the derived key.
	// The salt and the derived key are hex encoded.
	return fmt.Sprintf(
		`%d%s%x%s%x`,
		version,
		separator,
		salt,
		separator,
		derivedKey,
	), nil
}

// Eql performs a constant-time comparison between the password and the hash.
// The hash ought to have been produced by [Hash]
func Eql(password, hash string) error {
	params := strings.Split(hash, "$")

	if len(params) != 3 {
		return errors.New("unable to parse")
	}

	pVer, err := strconv.Atoi(params[0])
	if err != nil {
		return err
	}
	if pVer != version {
		return errors.New("version mismatch")
	}

	pSalt, err := hex.DecodeString(params[1])
	if err != nil {
		return err
	}

	pDerivedKey, err := hex.DecodeString(params[2])
	if err != nil {
		return err
	}

	dk, err := deriveKey([]byte(password), pSalt)
	if err != nil {
		return err
	}

	if subtle.ConstantTimeCompare(dk, pDerivedKey) == 1 {
		return nil
	}

	return errors.New("password mismatch")
}
