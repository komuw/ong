package finger

import "sync/atomic"

// TODO: docs.
type Print struct {
	Hash atomic.Pointer[string]
}
