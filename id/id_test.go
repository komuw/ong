// Package id is a unique id generator
package id

import (
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
			attest.True(t, a != b)
			attest.True(t, a != c)
		}

		{
			_len := 12
			a := Random(_len)
			b := Random(_len)
			c := Random(_len)
			attest.True(t, a != b)
			attest.True(t, a != c)
		}
	})
}
