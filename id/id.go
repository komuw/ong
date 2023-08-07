// Package id generates unique random identifiers.
package id

import (
	"crypto/rand"
	"encoding/base64"
)

// encodeURL is like [base64.EncodeURL] except we replace:
// (a) `-_` with `HQ`
// (b) `0,O,o` with `3,A,q`
// (c) `U,V,u,v` with `K,X,k,x`
// (d) `I,i,L,l,1` with `G,g,R,r,8`
// (e) `b,6` with `m,7`
const encodeURL = "ABCDEFGHGJKRMNAPQRSTKXWXYZamcdefghgjkrmnqpqrstkxwxyz3823457789HQ"

// encoding returns a [base64.Encoding] that is similar to [base64.RawURLEncoding] except that it uses [encodeURL]
func encoding() *base64.Encoding {
	return base64.NewEncoding(encodeURL).WithPadding(base64.NoPadding)
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

	// This formula is from [base64.Encoding.EncodedLen]
	byteSize := ((((n * 6) - 5) / 8) + 1)
	b := make([]byte, byteSize)

	if _, err := rand.Read(b); err != nil {
		panic(err)
	}

	return enc.EncodeToString(b)[:n]
}
