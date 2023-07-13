package id

import (
	"crypto/rand"
	"fmt"
	"time"
)

// Most of the code here is inspired(or taken from) by:
// (a) https://github.com/komuw/yuyuid whose license(MIT) can be found here: https://github.com/komuw/yuyuid/blob/v0.1.1/LICENSE.txt
// (b) https://github.com/gofrs/uuid whose license(MIT) can be found here:   https://github.com/gofrs/uuid/blob/v5.0.0/LICENSE
//

const (
	version4 byte = 4
	version7 byte = 7
)

// RFC's:
//   uuidv4: https://datatracker.ietf.org/doc/html/rfc4122
//   uuidv7: https://datatracker.ietf.org/doc/html/draft-ietf-uuidrev-rfc4122bis

// UUID represents a universally unique identifier.
// See [UUID4] and [UUID8]
//
// [unique]: https://en.wikipedia.org/wiki/Universally_unique_identifier
type UUID [16]byte

// String implements [fmt.Stringer] for uuid.
func (u UUID) String() string {
	return fmt.Sprintf(
		"%x-%x-%x-%x-%x",
		u[0:4], u[4:6], u[6:8], u[8:10], u[10:],
	)
}

// Bytes returns the bytes that undely the UUID.
func (u UUID) Bytes() []byte {
	return u[:]
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
// On the other hand, this means that it has poor database index locality unlike [UUID8]. It might also be a good use as a shard key.
//
// See [UUID8] and [New]
//
// It panics on error.
func UUID4() UUID {
	var uuid UUID

	// Probability of uuid collision;
	// https://towardsdatascience.com/are-uuids-really-unique-57eb80fc2a87

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

// UUID8 generates a version 8 [UUID].
// Version 8 [provides] an RFC-compatible format for experimental/specific use cases.
//
// This one is mostly like version 7 except it uses nanosecond precision instead of millisecond.
// It is correlated with timestamp, thus, when used as the identifief of an object, it has good database locality.
// On the other hand, this means that it can leak the object's creation time unlike [UUID4].
//
// See [UUID4] and [New]
//
// It panics on error.
//
// [provides]: https://datatracker.ietf.org/doc/html/draft-ietf-uuidrev-rfc4122bis#section-5.8
func UUID8() UUID {
	var uuid UUID
	// We are going to implement it mostly like a version7 uuid except we will use nanosecond precision.

	// Probability of uuid collision;
	// https://towardsdatascience.com/are-uuids-really-unique-57eb80fc2a87

	// Layout:
	// https://datatracker.ietf.org/doc/html/draft-ietf-uuidrev-rfc4122bis#section-5.7
	//
	// A UUID is 128 bits long & is intended to guarantee uniqueness across space and time.
	// | unix_ts_ms     | version | rand_a   | variant | rand_b   |
	// | 48bits(6bytes) | 4bits   | 12bits   | 2bits   | 62bits   |
	// | 0 - 47         | 48 - 51 | 52 - 63  | 64 - 65 | 66 - 127 |
	//

	// 1. Fill everything with randomness.
	//
	// Implementations SHOULD utilize a cryptographically secure pseudo-random number generator (CSPRNG) to provide values that are
	// both difficult to predict (unguessable) and have a low likelihood of collision (unique).
	// https://datatracker.ietf.org/doc/html/draft-ietf-uuidrev-rfc4122bis#section-6.8
	if _, err := rand.Read(uuid[:]); err != nil {
		panic(err)
	}

	// 2. Set the first 6bytes with time.
	//
	// It should be, big-endian unsigned number of Unix epoch timestamp in milliseconds.
	unix_ts_ms := uint64(time.Now().UTC().UnixNano())
	uuid[0] = byte(unix_ts_ms >> 40)
	uuid[1] = byte(unix_ts_ms >> 32)
	uuid[2] = byte(unix_ts_ms >> 24)
	uuid[3] = byte(unix_ts_ms >> 16)
	uuid[4] = byte(unix_ts_ms >> 8)
	uuid[5] = byte(unix_ts_ms)

	// 3. Override first 4bits of uuid[6]
	uuid.setVersion(version7)
	// 4. Override first 2bits of uuid[8]
	uuid.setVariant()

	return uuid
}
