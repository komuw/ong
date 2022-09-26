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
	version = 1
)

func deriveKey(password, salt []byte) (derivedKey []byte) {
	// derive a key.
	derivedKey, err := scrypt.Key(password, salt, n, r, p, keyLen)
	if err != nil {
		panic(err)
	}

	return derivedKey
}

func hash(password string) string {
	salt := random(saltLen, saltLen)
	derivedKey := deriveKey([]byte(password), salt)
	// Prepend the params and the salt to the derived key, each separated
	// by a "$" character. The salt and the derived key are hex encoded.
	return fmt.Sprintf(
		`%d$%x$%x`,
		version,
		salt,
		derivedKey,
	)
}

func eql(password, hash string) error {
	params := strings.Split(hash, "$")

	if len(params) != 3 {
		return errors.New("unable to parse")
	}

	pv, err := strconv.Atoi(params[0])
	if err != nil {
		return err
	}
	if pv != version {
		// TODO: better error messages
		return errors.New("unable to parse")
	}

	pSalt, err := hex.DecodeString(params[4])
	if err != nil {
		return err
	}

	pDerivedKey, err := hex.DecodeString(params[5])
	if err != nil {
		return err
	}

	dk := deriveKey([]byte(password), pSalt)

	if subtle.ConstantTimeCompare(dk, pDerivedKey) == 1 {
		return nil
	}

	return errors.New("password mismatch")
}
