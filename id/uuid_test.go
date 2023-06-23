package id

import (
	"testing"

	"go.akshayshah.org/attest"
)

func TestUuid(t *testing.T) {
	t.Parallel()

	t.Run("succeds", func(t *testing.T) {
		t.Parallel()

		v4 := UUID4()
		attest.NotZero(t, v4)

		v7 := UUID7()
		attest.NotZero(t, v7)
	})
}
