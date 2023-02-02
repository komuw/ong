package log

import (
	"context"
	"io"
	"sync"

	"github.com/komuw/ong/errors"
	"github.com/komuw/ong/id"
	"golang.org/x/exp/slog"
)

type logContextKeyType string

// CtxKey is the name of the context key used to store the logID.
const CtxKey = logContextKeyType("Ong-logID")

// usage:
//
//	glob := New(os.Stdout, 1_000)
//	ctx, span := tracer.Start(ctx, "myFuncName")
//	l := glob(ctx)
//	l.Info("hello world")
func New(w io.Writer, maxMsgs int) func(ctx context.Context) *slog.Logger {
	opts := slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
	}
	jh := opts.NewJSONHandler(w)
	cbuf := newCirleBuf(maxMsgs)
	h := handler{h: jh, cBuf: cbuf}
	l := slog.New(h)

	return func(ctx context.Context) *slog.Logger {
		return l.WithContext(ctx)
	}
}

// TODO: if we decide to use our own handler that is not backed by another(like JSONHandler)
//        we need to do our own locking.
// "User-defined handlers are responsible for their own locking."
// see: https://go-review.googlesource.com/c/exp/+/463255/2/slog/doc.go

// TODO: Make sure we have a way to get back an old school stdlib logger.
//       This is needed by things like http.Server.Errolog
// see: https://github.com/golang/go/issues/56345#issuecomment-1407635269
//
// we could add `Write()` method and a `GiveMeStdLogger()` method to `handler`
// then in ong/server/server.go we do;
// l := newLog.New(...)(ctx)
// if xl, ok := l.Handler().(ong/newlog.Handler); ok {
//    stdlibLogger = xl.GiveMeStdLogger()
//    http.Server.Errolog = stdlibLogger
//}

// custom handler.
// TODO: docs.
//
// This handler is similar to python's [memory handler]
//
// [memory handler]: https://github.com/python/cpython/blob/v3.11.1/Lib/logging/handlers.py#L1353-L1359
type handler struct {
	// from [slog.Handler] documentation:
	// Any of the Handler's methods may be called concurrently with itself or with other methods.
	// It is the responsibility of the Handler to manage this concurrency.
	h    slog.Handler
	cBuf *circleBuf
}

func (h handler) Enabled(_ context.Context, _ slog.Level) bool {
	return true /* support all logging levels*/
}

func (l handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &handler{h: l.h.WithAttrs(attrs)}
}

func (l handler) WithGroup(name string) slog.Handler {
	return &handler{h: l.h.WithGroup(name)}
}

func (l handler) Handle(r slog.Record) error {
	// TODO: make sure time is in UTC.
	// see: - https://github.com/golang/go/issues/56345#issuecomment-1407053167
	//      - https://github.com/golang/go/issues/56345#issuecomment-1406279161 (has an implementation)
	//
	id, _ := GetId(r.Context)
	newAttrs := []slog.Attr{
		{Key: "logID", Value: slog.StringValue(id)},
	}

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
					newAttrs = append(newAttrs, slog.Attr{Key: "stack", Value: slog.StringValue(stack)})
				}
			}
		}
	})
	r.AddAttrs(newAttrs...)

	// store record only after manipulating it.
	l.cBuf.store(r)

	var err error
	if r.Level >= slog.LevelError {
		l.cBuf.mu.Lock()
		for _, v := range l.cBuf.buf {
			if e := l.h.Handle(v); e != nil {
				err = e
			}
		}
		l.cBuf.mu.Unlock()
		l.cBuf.reset()
	}

	return err
}

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
