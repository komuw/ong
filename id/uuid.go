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

// RFC's:
//   uuidv4: https://datatracker.ietf.org/doc/html/rfc4122
//   uuidv7: https://datatracker.ietf.org/doc/html/draft-ietf-uuidrev-rfc4122bis

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
// It is not correlated with timestamp, thus, when used as the identifief of an object, it does not leak its creation time.
// On the other hand, this means that it has poor database index locality unlike [UUID7].
//
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

// UUID4 generates a version 7 [UUID].
// It is correlated with timestamp, thus, when used as the identifief of an object, it has good database locality.
// On the other hand, this that it can leak the object's creation time unlike [UUID4].
//
// It panics on error.
func UUID7() UUID {
	var uuid UUID
	return uuid
}
