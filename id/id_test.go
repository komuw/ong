package id

import (
	"math"
	"testing"

	"github.com/akshayjshah/attest"
)

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("succeds", func(t *testing.T) {
		t.Parallel()

		{
			got := New()
			attest.NotZero(t, got)

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
}
