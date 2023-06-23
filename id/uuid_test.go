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
		attest.NotZero(t, v4.Bytes())

		v8 := UUID8()
		attest.NotZero(t, v8)
		attest.NotZero(t, v8.Bytes())
	})

	t.Run("setVariant", func(t *testing.T) {
		t.Parallel()

		var uuid UUID
		uuid.setVariant()
		attest.NotZero(t, uuid)
		set := false
		for _, v := range uuid {
			if v != byte(0) {
				set = true
			}
		}
		attest.True(t, set)
	})

	t.Run("setVersion", func(t *testing.T) {
		var uuid UUID
		uuid.setVersion(version4)
		attest.NotZero(t, uuid)
		set := false
		for _, v := range uuid {
			if v != byte(0) {
				set = true
			}
		}
		attest.True(t, set)
	})
}
