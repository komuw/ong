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
var customEncodeURL = "ABCDEFGHGJKRMNAPQRSTKXWXYZamcdefghgjkrmnqpqrstkxwxyz3823457789HQ"

// customEncoding is like `base64.RawURLEncoding` except that it uses customEncodeURL
var customEncoding = base64.NewEncoding(customEncodeURL).WithPadding(base64.NoPadding)

var mathRandFromTime = mathRand.New(mathRand.NewSource(time.Now().UnixNano()))

// New returns a new random string
func New() string {
	return Random(16)
}

// Random generates a random string of size n.
// It uses `crypto/rand` but falls back to `math/rand` on error.
func Random(n int) string {
	// django appears to use 32 random characters for its csrf token.
	// so does gorilla/csrf; https://github.com/gorilla/csrf/blob/v1.7.1/csrf.go#L13-L14

	b := make([]byte, n)
	if _, err := cryptoRand.Read(b); err != nil {
		b = make([]byte, n)
		_, _ = mathRandFromTime.Read(b) // docs say that it always returns a nil error.
	}

	return customEncoding.EncodeToString(b)
}
