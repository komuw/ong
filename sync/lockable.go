package sync

import (
	"sync"
)

type Lockable[T any] struct {
	mu    sync.Mutex
	value T
}

func NewLockable[T any](value T) *Lockable[T] {
	return &Lockable[T]{value: value}
}

// Do calls [f] and then returns the latest value in [Lockable] and any error from calling [f].
// Note: even where [f] returns an error the returned [value] may not be the zero value. It will be the latest value in [Lockable]
func (l *Lockable[T]) Do(
	f func(oldValue T) (newValue T, _ error),
) (effectiveValue T, _ error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	newVal, err := f(l.value)
	if err == nil {
		l.value = newVal
	}
	val := l.value

	return val, err
}

// Get returns the latest value.
func (l *Lockable[T]) Get() (value T) {
	// the error here is guaranteed to e nil
	val, _ := l.Do(func(oldValue T) (newValue T, _ error) {
		return oldValue, nil
	})

	return val
}
