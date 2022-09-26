package cry

import (
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

func deriveKey(password []byte) (derivedKey, salt []byte) {
	// derive a key.
	salt = random(saltLen, saltLen) // should be random, 8 bytes is a good length.
	derivedKey, err := scrypt.Key(password, salt, n, r, p, keyLen)
	if err != nil {
		panic(err)
	}

	return derivedKey, salt
}

func hash(password string) string {
	derivedKey, salt := deriveKey([]byte(password))
	// Prepend the params and the salt to the derived key, each separated
	// by a "$" character. The salt and the derived key are hex encoded.
	return fmt.Sprintf(
		`%d$%d$%d$%d$%x$%x`,
		version,
		n,
		r,
		p,
		salt,
		derivedKey,
	)
}

func eql(password, hash string) error {
	params := strings.Split(hash, "$")

	if len(params) != 6 {
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

	pn, err := strconv.Atoi(params[1])
	if err != nil {
		return err
	}

	pr, err := strconv.Atoi(params[2])
	if err != nil {
		return err
	}

	pp, err := strconv.Atoi(params[3])
	if err != nil {
		return err
	}

	pSalt := params[4]
	pDerivedKey := params[5]
}
