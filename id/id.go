// Package id generates unique random identifiers.
package id

import (
	"crypto/rand"
	"encoding/base32"
)

// encodeAlphabet is like the alphabet used in [base32.StdEncoding] except we replace:
// (a) `O` with `q`
// (b) `U` with `a`
// (c) `V` with `r`
// (d) `I` with `m`
// (e) `L` with `e`
// (f) `K` with `d`
// (g) `6` with `h`
// This is done to try and reduce ambiguity.
const encodeAlphabet = "ABCDEFGHmJdeMNqPQRSTarWXYZ2345h7"

// encoding returns a [base32.Encoding] that is similar to [base32.StdPadding] except that it uses [encodeAlphabet]
func encoding() *base32.Encoding {
	return base32.NewEncoding(encodeAlphabet).WithPadding(base32.NoPadding)
}

var enc = encoding() //nolint:gochecknoglobals

// New returns a new random string consisting of a legible character set.
// It uses [rand]
//
// Also see [UUID4] and [UUID8]
//
// It panics on error.
func New() string {
	return Random(16)
}

// Random generates a random string of size n consisting of a legible character set.
// If n < 1 or significantly large, it is set to reasonable bounds.
// It uses [rand]
//
// Also see [UUID4] and [UUID8]
//
// It panics on error.
func Random(n int) string {
	if n < 1 {
		n = 1
	}
	if n > 100_000 {
		// the upper limit of a slice is some significant fraction of the address space of a process.
		// https://github.com/golang/go/issues/38673#issuecomment-643885108
		n = 100_000
	}

	// This formula is from [base32.Encoding.EncodedLen]
	byteSize := ((((n * 6) - 5) / 8) + 1)
	b := make([]byte, byteSize)

	if _, err := rand.Read(b); err != nil {
		panic(err)
	}

	return enc.EncodeToString(b)[:n]
}
