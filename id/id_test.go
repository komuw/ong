package id

import (
	"math"
	"testing"

	"github.com/akshayjshah/attest"
	"go.uber.org/goleak"
	"golang.org/x/exp/slices"
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
				t.Fatal("Random produced duplicates")
			} else {
				seen = append(seen, got)
			}
		}
	})
}
