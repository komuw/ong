package cry

import (
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Some of the code here is inspired by(or taken from):
//   (a) https://github.com/elithrar/simple-scrypt whose license(MIT) can be found here: https://github.com/elithrar/simple-scrypt/blob/v1.3.0/LICENSE

const (
	// this should be increased every time the parameters passed to [argon2.IDKey] are changed.
	version   = 2
	separator = "$"

	// The values recommended are:
	// golang.org/x/crypto/argon2
	_time   = uint32(1)
	memory  = uint32(64 * 1024) // 64MB
	threads = uint8(2)          // can be set to number of available CPUS. Can't use `runtime.NumCPU()` since that will be different between computers.
)

func deriveKey(password, salt []byte) []byte {
	// IDKey is Argon2id
	return argon2.IDKey(password, salt, _time, memory, threads, keyLen)
}

// Hash returns the argon2id hash of the password.
// It is safe to persist the result in your database instead of storing the actual password.
func Hash(password string) string {
	salt := random(saltLen, saltLen)
	derivedKey := deriveKey([]byte(password), salt)

	// Add version, salt to the derived key.
	// The salt and the derived key are hex encoded.
	return fmt.Sprintf(
		`%d%s%x%s%x`,
		version,
		separator,
		salt,
		separator,
		derivedKey,
	)
}

// Eql performs a constant-time comparison between the password and the hash.
// The hash ought to have been produced by [Hash]
func Eql(password, hash string) error {
	params := strings.Split(hash, "$")

	fmt.Println("\n\t params: ", params)
	if len(params) != 3 {
		return errors.New("ong/cry: unable to parse")
	}

	pVer, err := strconv.Atoi(params[0])
	if err != nil {
		return err
	}
	if pVer != version {
		return errors.New("ong/cry: version mismatch")
	}

	pSalt, err := hex.DecodeString(params[1])
	if err != nil {
		return err
	}

	pDerivedKey, err := hex.DecodeString(params[2])
	if err != nil {
		return err
	}

	dk := deriveKey([]byte(password), pSalt)
	if subtle.ConstantTimeCompare(dk, pDerivedKey) == 1 {
		return nil
	}

	return errors.New("ong/cry: password mismatch")
}
