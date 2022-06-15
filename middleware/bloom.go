package middleware

import (
	"fmt"
	"hash/maphash"
)

// bloom implements a simple bloom filter.
// A Bloom filter is a probabilistic data structure that tells you, efficiently, whether an element is present in a set.
// It tells us whether an element is;
// - DEFINITELY NOT in the set, or
// - may be in the set.
// With more elements in the bloom, the probability of false positives increases.
type bloom struct {
	size uint64
	// todo: re-implement using actual bitsets for more efficiency & speed.
	bitArray  []uint64
	hashCount uint8
	h         maphash.Hash
}

func newBloom(size uint64, hashCount uint8) *bloom {
	return &bloom{
		size:      size,
		bitArray:  make([]uint64, size, size),
		hashCount: hashCount,
		h:         maphash.Hash{},
	}
}

func (b *bloom) set(item string) {
	for i := 0; i <= int(b.hashCount); i++ {
		index := b.hash(item + fmt.Sprint(i))
		b.bitArray[index] = 1
	}
}

func (b *bloom) get(item string) bool {
	present := true
	for i := 0; i <= int(b.hashCount); i++ {
		index := b.hash(item + fmt.Sprint(i))
		if b.bitArray[index] == 0 {
			present = false
			return present
		}
	}
	return present
}

func (b *bloom) hash(item string) uint64 {
	b.h.Reset()
	// WriteString never fails, the two return values are just so it fulfills io.StringWriter interface.
	_, _ = b.h.WriteString(item)
	index := b.h.Sum64() % b.size
	return index
}

func (b *bloom) reset() {
	b.bitArray = make([]uint64, b.size, b.size)
}
