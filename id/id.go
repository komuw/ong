// Package id generates unique random identifiers.
package id

import (
	"math/rand/v2"
)

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

# Collisions:
If there are n objects available from which to select, and permutations (P) are to be formed using k of the objects at a time,
the number of different permutations possible is denoted by the symbol nPk. A formula for its evaluation is;
nPk = n!/(n − k)!. Note ! is factorial.
If you have 5 objects(A, B, C, D, E) and we select 2 objects, the number of permutations(NOT combinations) is;
nPk = 5!/(5-2)! == 120/6 = 20

Permutations allows repetitions whereas combinations do not. In permutation, AB & BA are distinct.
*/
const alphabet = "ABEGHNQRTadeghnqrt2345789"

// https://www.britannica.com/science/permutation

// New returns a new random string consisting of a legible character set.
// It is not suitable for cryptographic uses.
//
// Also see [UUID4] and [UUID8]
func New() string {
	return Random(16)
}

// Random generates a random string of size n consisting of a legible character set.
// If n < 1 or significantly large, it is set to reasonable bounds.
// It is not suitable for cryptographic uses.
//
// Also see [UUID4] and [UUID8]
func Random(n int) string {
	if n < 1 {
		n = 1
	}
	if n > 100_000 {
		// the upper limit of a slice is some significant fraction of the address space of a process.
		// https://github.com/golang/go/issues/38673#issuecomment-643885108
		n = 100_000
	}

	length := len(alphabet)
	b := make([]byte, n)

	for i := range b {
		j := rand.N(length)
		b[i] = alphabet[j]
	}

	return string(b)
}
