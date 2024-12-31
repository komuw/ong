package sync_test

import (
	"fmt"
	"testing"

	"github.com/komuw/ong/sync"
	"go.akshayshah.org/attest"
)

func TestLockable(t *testing.T) {
	t.Parallel()

	log := sync.NewLockable(0)

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		attest.Equal(t, log.Get(), 0)

		val, err := log.Do(
			func(oldval int) (int, error) {
				return 34, nil
			},
		)
		attest.Ok(t, err)
		attest.Equal(t, val, 34)
		attest.Equal(t, log.Get(), val)

		_ = val
		fmt.Println("val: ", log.Get())

		// for i := range 19 {
		// 	go func(ii int) {
		// 		val, err := log.Do(
		// 			func(l *Lockable[int]) error {
		// 				l.value = ii
		// 				return nil
		// 			},
		// 		)
		// 		if err != nil {
		// 			panic(err)
		// 		}
		// 		fmt.Println("ii: ", ii, "val: ", val)
		// 	}(i)
		// }
	})
}
