package id

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"time"
)

// Most of the code here is inspired(or taken from) by:
// (a) https://github.com/komuw/yuyuid whose license(MIT) can be found here: https://github.com/komuw/yuyuid/blob/v0.1.1/LICENSE.txt
// (b) https://github.com/gofrs/uuid whose license(MIT) can be found here:   https://github.com/gofrs/uuid/blob/v5.0.0/LICENSE
//

const (
	reservedNcs       byte = 0x80 // Reserved for NCS compatibility
	rfc4122           byte = 0x40 // Specified in RFC 4122
	reservedMicrosoft byte = 0x20 // Reserved for Microsoft compatibility
	reservedFuture    byte = 0x00 // Reserved for future definition.

	version4 byte = 4
	version7 byte = 7
)

// RFC's:
//   uuidv4: https://datatracker.ietf.org/doc/html/rfc4122
//   uuidv7: https://datatracker.ietf.org/doc/html/draft-ietf-uuidrev-rfc4122bis

// UUID represents a universally unique identifier.
// See [UUID4] and [UUID7]
//
// [unique]: https://en.wikipedia.org/wiki/Universally_unique_identifier
type UUID [16]byte

// TODO: look into endianess.

func (u UUID) String() string {
	return fmt.Sprintf(
		"%x-%x-%x-%x-%x",
		u[0:4], u[4:6], u[6:8], u[8:10], u[10:],
	)
}

func (u *UUID) setVariant() {
	// In this package, we only use rfc4122 variant.
	u[8] = (u[8]&(0xff>>2) | (0x02 << 6))
}

func (u *UUID) setVersion(version byte) {
	u[6] = (u[6] & 0x0f) | (version << 4)
}

// UUID4 generates a version 4 [UUID].
// It is not correlated with timestamp, thus, when used as the identifief of an object, it does not leak its creation time.
// On the other hand, this means that it has poor database index locality unlike [UUID7].
//
// It panics on error.
func UUID4() UUID {
	var uuid UUID

	// Layout:
	// https://datatracker.ietf.org/doc/html/draft-ietf-uuidrev-rfc4122bis#section-5.4
	//
	// A UUID is 128 bits long & is intended to guarantee uniqueness across space and time.
	// | random_a       | version | random_b | variant | random_c |
	// | 48bits(6bytes) | 4bits   | 12bits   | 2bits   | 62bits   |
	// | 0 - 47         | 48 - 51 | 52 - 63  | 64 - 65 | 66 - 127 |
	//

	// Read is a helper function that calls io.ReadFull.
	//
	// Implementations SHOULD utilize a cryptographically secure pseudo-random number generator (CSPRNG) to provide values that are
	// both difficult to predict (unguessable) and have a low likelihood of collision (unique).
	// https://datatracker.ietf.org/doc/html/draft-ietf-uuidrev-rfc4122bis#section-6.8
	if _, err := rand.Read(uuid[:]); err != nil {
		panic(err)
	}

	uuid.setVersion(version4)
	uuid.setVariant()

	return uuid
}

// UUID4 generates a version 7 [UUID].
// It is correlated with timestamp, thus, when used as the identifief of an object, it has good database locality.
// On the other hand, this that it can leak the object's creation time unlike [UUID4].
//
// It panics on error.
func UUID7() UUID {
	var uuid UUID

	// Layout:
	// https://datatracker.ietf.org/doc/html/draft-ietf-uuidrev-rfc4122bis#section-5.7
	//
	// A UUID is 128 bits long & is intended to guarantee uniqueness across space and time.
	// | unix_ts_ms     | version | rand_a   | variant | rand_b   |
	// | 48bits(6bytes) | 4bits   | 12bits   | 2bits   | 62bits   |
	// | 0 - 47         | 48 - 51 | 52 - 63  | 64 - 65 | 66 - 127 |
	//

	// It should be;
	// big-endian unsigned number of Unix epoch timestamp in milliseconds.
	unix_ts_ms := uint64(time.Now().UTC().UnixMilli())
	binary.BigEndian.PutUint64(uuid[:6], unix_ts_ms)
	uuid.setVersion(version7)

	return uuid
}
