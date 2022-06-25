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

		got := New()
		attest.NotZero(t, got)

		a := Random(12)
		b := Random(12)
		c := Random(12)

		attest.True(t, a != b)
		attest.True(t, a != c)
	})
}
