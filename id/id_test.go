package id

import (
	"math"
	"slices"
	"testing"
	"unsafe"

	"go.akshayshah.org/attest"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	// call flag.Parse() here if TestMain uses flags
	goleak.VerifyTestMain(m)
}

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("succeds", func(t *testing.T) {
		t.Parallel()

		{
			got := New()
			attest.NotZero(t, got)
			attest.Equal(t, len(got), 16)

			a := New()
			b := New()
			c := New()
			attest.NotEqual(t, a, b)
			attest.NotEqual(t, a, c)
		}

		{
			_len := 12
			a := Random(_len)
			b := Random(_len)
			c := Random(_len)
			attest.NotEqual(t, a, b)
			attest.NotEqual(t, a, c)

			attest.Equal(t, len(c), _len)
		}

		{
			got := Random(-1)
			attest.NotZero(t, got)

			got = Random(-92)
			attest.NotZero(t, got)

			got = Random(0)
			attest.NotZero(t, got)

			got = Random(1)
			attest.NotZero(t, got)

			got = Random(math.MaxInt)
			attest.NotZero(t, got)
		}
	})

	t.Run("range", func(t *testing.T) {
		t.Parallel()

		for i := 1; i <= 10_001; i++ {
			got := Random(i)
			attest.NotZero(t, got)
			_len := len(got)
			attest.Equal(t, _len, i, attest.Sprintf("input(%d), got len(%d) ", i, _len))
		}
	})

	t.Run("no dupes", func(t *testing.T) {
		t.Parallel()

		seen := []string{}
		for i := 1; i <= 10_001; i++ {
			got := New()
			attest.NotZero(t, got)
			_len := len(got)
			attest.Equal(t, _len, 16, attest.Sprintf("input(%d), got len(%d) ", i, _len))

			if slices.Contains(seen, got) {
				t.Fatal("New produced duplicates")
			} else {
				seen = append(seen, got)
			}
		}
	})

	t.Run("permutation of new", func(t *testing.T) {
		t.Parallel()

		{
			n := 5
			k := 2
			nPk := factorial(n) / factorial((n - k))
			attest.Equal(t, nPk, 20)
		}

		{
			n := len(alphabet)
			k := len(New())
			permutation := factorial(n) / factorial((n - k))
			attest.Equal(t, permutation, 19_385_293_423_649) // ~19 trillion
		}

		{
			// The birthday-paradox places an upper bound on collision resistance;
			// If a hash function produces N bits of output, an attacker who computes √2^N hash operations on random input is likely to find two matching outputs.
			// https://algo.komu.engineer/6_hashmaps
			sizeBytes := unsafe.Sizeof(New())
			sizeBits := int(sizeBytes * 8)
			pow := math.Exp2(float64(sizeBits))
			numOpsB4Collision := math.Sqrt(pow)
			attest.True(t, numOpsB4Collision > 1.84e+19) // You would have to generate that many before colllisions occur.
			attest.True(t, numOpsB4Collision < 1.85e+19)
			attest.True(t, numOpsB4Collision > 900_000_000_000_000) // 900 trillion
		}
	})
}

func factorial(num int) int {
	if num == 1 || num == 0 {
		return num
	}
	return num * factorial(num-1)
}
