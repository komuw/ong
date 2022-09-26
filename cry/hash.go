package cry

import (
	"fmt"

	"golang.org/x/crypto/scrypt"
)

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
