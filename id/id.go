// Package id is a unique id generator
package id

import (
	cryptoRand "crypto/rand"
	"encoding/base64"
	mathRand "math/rand"
	"time"
)

/*
customEncodeURL is like `bas64.customEncodeURL` except we replace:
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
// It uses `crypto/rand` but falls back to `math/rand` on error.
func Random(n int) string {
	b := make([]byte, n)
	if _, err := cryptoRand.Read(b); err != nil {
		b = make([]byte, n)
		mathRandFromTime := mathRand.New(
			// this codepath is rarely executed so we dont need to put `mathRandFromTime` as a global var.
			mathRand.NewSource(time.Now().UnixNano()),
		)
		_, _ = mathRandFromTime.Read(b) // docs say that it always returns a nil error.
	}

	return customEncoding.EncodeToString(b)
}
