package id

import (
	"fmt"
	"sort"
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

	t.Run("uuid8 is sortable", func(t *testing.T) {
		t.Parallel()

		first, last := "", ""
		s := []string{}
		for i := range 10 {
			v8 := UUID8()
			attest.NotZero(t, v8)
			attest.NotZero(t, v8.Bytes())

			fmt.Println("\t v8: ", v8)
			s = append(s, v8.String())
			if i == 0 {
				first = v8.String()
			}
			if i == 9 {
				last = v8.String()
			}
		}

		sort.Strings(s)
		attest.Equal(t, first, s[0])
		attest.Equal(t, last, s[9])
	})
}
