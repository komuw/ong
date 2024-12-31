package sync_test

import (
	"fmt"
	"testing"

	"github.com/komuw/ong/sync"
)

func TestLockable(t *testing.T) {
	t.Parallel()

	log := sync.NewLockable(0)

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		fmt.Println("val: ", log.Get())
		val, err := log.Do(
			func(oldval int) (int, error) {
				return 34, nil
			},
		)
		if err != nil {
			panic(err)
		}

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
