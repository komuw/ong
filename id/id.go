// Package id generates unique random identifiers.
package id

import "crypto/rand"

/*
alphabet is similar to the alphabet used in [base64.URLEncoding] except we remove:
-_            : They are not pronounceable.
C, c          : They both look similar to each other.
D, 0, O, o    : They both look similar to each other.
F, f          : They both look similar to each other.
I, i, 1, L, l : They both look similar to each other.
J, j          : They both look similar to each other.
K, k          : They both look similar to each other.
M, m          : They both look similar to each other.
P, p          : They both look similar to each other.
S, s          : They both look similar to each other.
U, u, V, v    : They both look similar to each other.
W,w           : They both look similar to each other.
X,x           : They both look similar to each other.
Y,y           : They both look similar to each other.
Z,z           : They both look similar to each other.
6, b          : They both look similar to each other.

This is done to try and reduce ambiguity.

Each character is selected independently and can repeat.
An n-character result has len(alphabet)ⁿ possible values.
*/
const alphabet = "ABEGHNQRTadeghnqrt2345789"

// New returns a cryptographically random string consisting of a legible character set.
// The result contains at least 128 bits of randomness.
//
// Also see [UUID4] and [UUID8]
func New() string {
	// ⌈log₂₅ 2¹²⁸⌉ = 28 chars.
	// Also see [rand.Text]
	return Random(28)
}

// Random generates a cryptographically random string of size n consisting of a legible character set.
// If n < 1 or significantly large, it is set to reasonable bounds.
// The security of the result depends on n.
//
// Also see [UUID4] and [UUID8]
func Random(n int) string {
	if n < 1 {
		n = 6
	}
	if n > 100_000 {
		// the upper limit of a slice is some significant fraction of the address space of a process.
		// https://github.com/golang/go/issues/38673#issuecomment-643885108
		n = 100_000
	}

	// Reject the incomplete range before using modulo to keep each character equally likely.
	const randomByteLimit = 256 - (256 % len(alphabet))

	b := make([]byte, n)
	alphabetLength := len(alphabet)

	for i := 0; i < n; {
		randomBytes := b[i:]
		_, _ = rand.Read(randomBytes)

		for _, randomByte := range randomBytes {
			if int(randomByte) >= randomByteLimit {
				continue
			}

			b[i] = alphabet[int(randomByte)%alphabetLength]
			i++
		}
	}

	return string(b)
}
