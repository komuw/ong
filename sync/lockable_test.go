package sync_test

import (
	stdlibSync "sync"
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
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		age := sync.NewLockable(0)

		runhandler := func(i int) {
			val, err := age.Do(
				func(oldval int) (int, error) {
					return i, nil
				},
			)
			attest.Ok(t, err)
			attest.Equal(t, val, i)
			attest.Equal(t, log.Get(), val)
		}

		wg := &stdlibSync.WaitGroup{}
		for rN := 0; rN <= 34; rN++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				runhandler(i)
			}(rN)
		}
		wg.Wait()
	})
}
