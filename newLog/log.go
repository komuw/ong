package log

import (
	"context"
	"io"
	"sync"

	"github.com/komuw/ong/errors"
	"github.com/komuw/ong/id"
	"golang.org/x/exp/slog"
)

// usage:
//
//	glob := NewSlog(os.Stdout, 1_000)
//	ctx, span := tracer.Start(ctx, "myFuncName")
//	l := glob(ctx)
//	l.Info("hello world")
func NewSlog(w io.Writer, maxMsgs int) func(ctx context.Context) *slog.Logger {
	opts := slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
	}
	jh := opts.NewJSONHandler(w)
	cbuf := newCirleBuf(maxMsgs)
	h := cHandler{h: jh, cBuf: cbuf}

	return func(ctx context.Context) *slog.Logger {
		l := slog.New(h)

		return l.WithContext(ctx)
	}
}

// TODO: if we decide to use our own handler that is not backed by another(like JSONHandler)
//        we need to do our own locking.
// "User-defined handlers are responsible for their own locking."
// see: https://go-review.googlesource.com/c/exp/+/463255/2/slog/doc.go

// custom handler.
type cHandler struct {
	h    slog.Handler
	cBuf *circleBuf
}

func (s cHandler) Enabled(_ slog.Level) bool { return true /* support all logging levels*/ }
func (s cHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &cHandler{h: s.h.WithAttrs(attrs)}
}

func (s cHandler) WithGroup(name string) slog.Handler {
	return &cHandler{h: s.h.WithGroup(name)}
}

// TODO: rename receiver from `s`
func (s cHandler) Handle(r slog.Record) error {
	// TODO: make sure time is in UTC.
	// see: https://github.com/golang/go/issues/56345#issuecomment-1407053167
	id, _ := GetId(r.Context)
	// TODO: we should only call `r.AddAttrs` once in this entire method.
	r.AddAttrs(slog.Attr{Key: "logID", Value: slog.StringValue(id)})

	// TODO: Obey the following rules form the slog documentation:
	//
	// Handle methods that produce output should observe the following rules:
	//   - If r.Time is the zero time, ignore the time.
	//   - If an Attr's key is the empty string, ignore the Attr.
	//
	r.Attrs(func(a slog.Attr) {
		if a.Key == slog.ErrorKey {
			if e, ok := a.Value.Any().(error); ok {
				if stack := errors.StackTrace(e); stack != "" {
					r.AddAttrs(slog.Attr{Key: "stack", Value: slog.StringValue(stack)})
				}
			}
		}
	})

	// store record only after manipulating it.
	s.cBuf.store(r)

	var err error
	if r.Level >= slog.LevelError {
		s.cBuf.mu.Lock()
		for _, v := range s.cBuf.buf {
			// TODO: check how it handles special characters
			// see: https://github.com/komuw/ong/commit/fd94ed712d9baa5b42d5ff16f1fe561337491328
			if e := s.h.Handle(v); e != nil {
				err = e
			}
		}
		s.cBuf.mu.Unlock()
		s.cBuf.reset()
	}

	return err
}

type logContextKeyType string

const // CtxKey is the name of the context key used to store the logID.
CtxKey = logContextKeyType("Ong-logID")

// GetId gets a logId either from the provided context or auto-generated.
// It returns the logID and true if the id came from ctx else false
func GetId(ctx context.Context) (string, bool) {
	if ctx != nil {
		if vCtx := ctx.Value(CtxKey); vCtx != nil {
			if s, ok := vCtx.(string); ok {
				return s, true
			}
		}
	}
	return id.New(), false
}

// circleBuf implements a very simple & naive circular buffer.
type circleBuf struct {
	mu      sync.Mutex // protects buf
	buf     []slog.Record
	maxSize int
}

func newCirleBuf(maxSize int) *circleBuf {
	if maxSize <= 0 {
		maxSize = 10
	}
	c := &circleBuf{
		buf:     make([]slog.Record, maxSize),
		maxSize: maxSize,
	}
	c.reset() // remove the nils from `make()`
	return c
}

func (c *circleBuf) store(r slog.Record) {
	c.mu.Lock()
	defer c.mu.Unlock()

	availableSpace := c.maxSize - len(c.buf)
	if availableSpace <= 0 {
		// clear space.
		//
		// Here, we clear a quarter of the slice. This means also some of the earlier items may never be cleared.
		// If maxSize==7, when we get to this part of code upto == (7/4 == 1)
		// so resulting buf == c.buf[:1], which will retain the first element.
		// This first element will never be cleared unless `.reset` is called.
		// see: https://go.dev/play/p/u7qWWt1C7oc
		upto := c.maxSize / 4
		c.buf = c.buf[:upto]
	}

	c.buf = append(c.buf, r)
}

func (c *circleBuf) reset() {
	c.mu.Lock()
	c.buf = c.buf[:0]
	c.mu.Unlock()
}
