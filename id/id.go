// Package id is a unique id generator
package id

import (
	cryptoRand "crypto/rand"
	"encoding/base64"
	mathRand "math/rand"
	"time"
)

/*
customEncodeURL is like `bas64.encodeURL` except we replace:
	(a) `-_` with `HQ`
	(b) `0,O,o` with `3,A,q`
	(c) `U,V,u,v` with `K,X,k,x`
	(d) `I,i,L,l,1` with `G,g,R,r,8`
	(e) `b,6` with `m,7`
*/
const customEncodeURL = "ABCDEFGHGJKRMNAPQRSTKXWXYZamcdefghgjkrmnqpqrstkxwxyz3823457789HQ"

// customEncoding is like `base64.RawURLEncoding` except that it uses customEncodeURL
var customEncoding = base64.NewEncoding(customEncodeURL).WithPadding(base64.NoPadding) //nolint:gochecknoglobals

// New returns a new random string
func New() string {
	return Random(16)
}

// Random generates a random string made from bytes of size n.
// If n < 1 or significantly large, it is set to reasonable bounds.
// It uses `crypto/rand` but falls back to `math/rand` on error.
func Random(n int) string {
	if n < 1 {
		n = 1
	}
	if n > 100_000 {
		// the upper limit of a slice is some significant fraction of the address space of a process.
		// https://github.com/golang/go/issues/38673#issuecomment-643885108
		n = 100_000
	}

	b := make([]byte, n)
	if _, err := cryptoRand.Read(b); err != nil {
		b = make([]byte, n)
		// this codepath is rarely executed so we can put all the code here instead of global var.
		mathRand.Seed(time.Now().UTC().UnixNano())
		_, _ = mathRand.Read(b) // docs say that it always returns a nil error.
	}

	return customEncoding.EncodeToString(b)
}
