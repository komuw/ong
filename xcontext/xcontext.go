// Package xcontext is a package to offer the extra functionality we need
// from contexts that is not available from the standard context package.
package xcontext

import (
	"context"
	"time"
)

// Most of the code here is insipired by(or taken from):
//   (a) https://github.com/golang/pkgsite whose license(BSD 3-Clause "New") can be found here: https://github.com/golang/pkgsite/blob/24f94ffc546bde6aae0552efa6a940041d9d28e1/LICENSE
//

// Note: There's a Go proposal to bring something like into the std lib:
//       https://github.com/golang/go/issues/40221
//

// Detach returns a context that keeps all the values of its parent context but detaches from the cancellation and error handling.
func Detach(ctx context.Context) context.Context { return detachedContext{ctx} }

type detachedContext struct{ parent context.Context }

func (d detachedContext) Deadline() (time.Time, bool)       { return time.Time{}, false }
func (d detachedContext) Done() <-chan struct{}             { return nil }
func (d detachedContext) Err() error                        { return nil }
func (d detachedContext) Value(key interface{}) interface{} { return d.parent.Value(key) }
