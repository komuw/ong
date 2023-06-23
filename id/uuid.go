package id

import (
	"crypto/rand"
	"fmt"
)

// Most of the code here is inspired(or taken from) by:
// (a) https://github.com/komuw/yuyuid whose license(MIT) can be found here: https://github.com/komuw/yuyuid/blob/v0.1.1/LICENSE.txt
//

const (
	reservedNcs       byte = 0x80 // Reserved for NCS compatibility
	rfc4122           byte = 0x40 // Specified in RFC 4122
	reservedMicrosoft byte = 0x20 // Reserved for Microsoft compatibility
	reservedFuture    byte = 0x00 // Reserved for future definition.

	version4 byte = 4
)

// UUID represents a universally unique identifier.
// See [UUID4] and [UUID7]
//
// [unique]: https://en.wikipedia.org/wiki/Universally_unique_identifier
type UUID [16]byte

func (u UUID) String() string {
	return fmt.Sprintf(
		"%x-%x-%x-%x-%x",
		u[0:4], u[4:6], u[6:8], u[8:10], u[10:],
	)
}

func (u *UUID) setVariant(variant byte) {
	switch variant {
	case reservedNcs:
		u[8] &= 0x7F
	case rfc4122:
		u[8] &= 0x3F
		u[8] |= 0x80
	case reservedMicrosoft:
		u[8] &= 0x1F
		u[8] |= 0xC0
	case reservedFuture:
		u[8] &= 0x1F
		u[8] |= 0xE0
	default:
		panic(fmt.Sprintf("variant: %v is unknown", variant))
	}
}

func (u *UUID) setVersion(version byte) {
	u[6] = (u[6] & 0x0F) | (version << 4)
}

// UUID4 generates a version 4 [UUID].
// It panics on error.
func UUID4() UUID {
	var uuid UUID

	// Read is a helper function that calls io.ReadFull.
	if _, err := rand.Read(uuid[:]); err != nil {
		panic(err)
	}

	uuid.setVariant(rfc4122)
	uuid.setVersion(version4)
	return uuid
}
