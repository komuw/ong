package middleware

import (
	"sync"
	"testing"

	"github.com/akshayjshah/attest"
)

func TestStore(t *testing.T) {
	t.Parallel()

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		store := NewMemStore()

		tokens := []string{
			"a", "aa", "aaa", "aaron", "ab", "abandoned", "abc", "aberdeen", "abilities", "ability", "able", "aboriginal", "abortion",
			"about", "above", "abraham", "abroad", "abs", "absence", "absent", "absolute", "absolutely", "absorption", "abstract",
			"abstracts", "abu", "abuse", "ac", "academic", "academics", "academy", "acc", "accent", "accept", "acceptable", "acceptance",
			"accepted", "accepting", "accepts", "access", "accessed", "accessibility", "accessible", "accessing", "accessories",
			"accessory", "accident", "accidents", "accommodate", "accommodation", "accommodations", "accompanied", "accompanying",
			"accomplish", "accomplished", "accordance", "according", "accordingly", "account", "accountability", "accounting", "accounts",
			"accreditation", "accredited", "accuracy", "accurate", "accurately", "accused", "acdbentity", "ace",
		}

		for _, tok := range tokens {
			go func(t string) {
				store.Set(t)
			}(tok)
		}

		for _, tok := range tokens {
			go func(t string) {
				store.Exists(t)
			}(tok)
		}

		for range tokens {
			go func() {
				store.Reset()
			}()
		}

		wg := &sync.WaitGroup{}
		for _, tok := range tokens {
			wg.Add(1)
			go func(t string) {
				defer wg.Done()
				store.Set(t)
			}(tok)
		}
		wg.Wait()
	})

	t.Run("set", func(t *testing.T) {
		t.Parallel()

		store := NewMemStore()

		tokens := []string{
			"a", "aa", "aaa", "aaron", "ab", "abandoned", "abc", "aberdeen", "abilities", "ability", "able", "aboriginal", "abortion",
			"about", "above", "abraham", "abroad", "abs", "absence", "absent", "absolute", "absolutely", "absorption", "abstract",
			"abstracts", "abu", "abuse", "ac", "academic", "academics", "academy", "acc", "accent", "accept", "acceptable", "acceptance",
			"accepted", "accepting", "accepts", "access", "accessed", "accessibility", "accessible", "accessing", "accessories",
			"accessory", "accident", "accidents", "accommodate", "accommodation", "accommodations", "accompanied", "accompanying",
			"accomplish", "accomplished", "accordance", "according", "accordingly", "account", "accountability", "accounting", "accounts",
			"accreditation", "accredited", "accuracy", "accurate", "accurately", "accused", "acdbentity", "ace",
		}
		wg := &sync.WaitGroup{}
		for _, tok := range tokens {
			wg.Add(1)
			go func(t string) {
				defer wg.Done()
				store.Set(t)
			}(tok)
		}
		wg.Wait()

		attest.Equal(t, store._len(), len(tokens))
	})

	t.Run("reset", func(t *testing.T) {
		t.Parallel()

		store := NewMemStore()

		tokens := []string{"aaron", "abandoned", "according", "accreditation", "accurately", "accused"}
		wg := &sync.WaitGroup{}
		for _, tok := range tokens {
			wg.Add(1)
			go func(t string) {
				defer wg.Done()
				store.Set(t)
			}(tok)
		}
		wg.Wait()

		attest.Equal(t, store._len(), len(tokens))

		store.Reset()
		attest.Equal(t, store._len(), 0)
	})
}
