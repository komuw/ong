// Package log implements a simple logging handler.
package log

import (
	"context"
	"io"
	stdLog "log"
	"runtime"
	"sync"
	"time"

	"github.com/komuw/ong/errors"
	"github.com/komuw/ong/id"
	"golang.org/x/exp/slog"
)

type logContextKeyType string

// CtxKey is the name of the context key used to store the logID.
const CtxKey = logContextKeyType("Ong-logID")

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

// New returns an [slog.Logger] that is backed by a handler that stores log messages into a [circular buffer].
// Those log messages are only flushed to the underlying io.Writer when a message with level >=[slog.LevelError] is logged.
// A unique logID is also added to the logs that acts as a correlation id of log events from diffrent components that
// neverthless are related.
//
// [circular buffer]: https://en.wikipedia.org/wiki/Circular_buffer
func New(w io.Writer, maxMsgs int) func(ctx context.Context) *slog.Logger {
	opts := slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
	}
	jh := opts.NewJSONHandler(w)
	cbuf := newCirleBuf(maxMsgs)
	h := Handler{h: jh, cBuf: cbuf, logID: id.New(), immediate: false}
	l := slog.New(h)

	return func(ctx context.Context) *slog.Logger {
		id := h.logID
		if id2, ok := GetId(ctx); ok {
			// if ctx did not contain a logId, do not use the generated one.
			id = id2
		}

		ctx = context.WithValue(ctx, CtxKey, id)
		return l.WithContext(ctx)
	}
}

// Handler is an [slog.Handler]
// It stores log messages into a [circular buffer]. Those log messages are only flushed to the underlying io.Writer when
// a message with level >=[slog.LevelError] is logged.
//
// [circular buffer]: https://en.wikipedia.org/wiki/Circular_buffer
type Handler struct {
	// This handler is similar to python's memory handler.
	// https://github.com/python/cpython/blob/v3.11.1/Lib/logging/handlers.py#L1353-L1359
	//
	// from [slog.Handler] documentation:
	// Any of the Handler's methods may be called concurrently with itself or with other methods.
	// It is the responsibility of the Handler to manage this concurrency.
	// https://go-review.googlesource.com/c/exp/+/463255/2/slog/doc.go
	h     slog.Handler
	cBuf  *circleBuf
	logID string

	// remove this once the following is implemnted
	// https://github.com/golang/go/issues/56345#issuecomment-1407635269
	immediate bool
}

func (h Handler) Enabled(_ context.Context, _ slog.Level) bool {
	return true /* support all logging levels*/
}

func (h Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return Handler{h: h.h.WithAttrs(attrs), cBuf: h.cBuf, logID: h.logID, immediate: h.immediate}
}

func (h Handler) WithGroup(name string) slog.Handler {
	return Handler{h: h.h.WithGroup(name), cBuf: h.cBuf, logID: h.logID, immediate: h.immediate}
}

func (h Handler) Handle(r slog.Record) error {
	// Obey the following rules form the slog documentation:
	// Handle methods that produce output should observe the following rules:
	//   - If r.Time is the zero time, ignore the time.
	//   - If an Attr's key is the empty string, ignore the Attr.

	// Convert time to UTC.
	// Note that we do not convert any other fields(that may be of type time.Time) into UTC.
	// If we ever need that functionality, we would do that in `r.Attrs()`
	if r.Time.IsZero() {
		r.Time = time.Now().UTC()
	}
	r.Time = r.Time.UTC()

	id := h.logID
	if id2, ok := GetId(r.Context); ok {
		// if ctx did not contain a logId, do not use the generated one.
		id = id2
	}
	r.Context = context.WithValue(r.Context, CtxKey, id)

	newAttrs := []slog.Attr{
		{Key: "logID", Value: slog.StringValue(id)},
	}

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

	h.cBuf.mu.Lock()
	defer h.cBuf.mu.Unlock()

	// store record only after manipulating it.
	h.cBuf.store(r)

	var err error
	if r.Level >= slog.LevelError {
		for _, v := range h.cBuf.buf {
			if e := h.h.Handle(v); e != nil {
				err = e
			}
		}
		h.cBuf.reset()
	} else if h.immediate {
		// remove once the following is implemnted.
		// https://github.com/golang/go/issues/56345#issuecomment-1407635269
		err = h.h.Handle(r)
	}

	return err
}

// TODO: Remove the `handler.Write` and `handler.StdLogger` methods.
//       Also make `Handler` private
//       This is needed by things like http.Server.Errolog
// see: https://github.com/golang/go/issues/56345#issuecomment-1407635269

// StdLogger returns an unstructured logger from the Go standard library log package.
// That logger will use l as its output.
func (h Handler) StdLogger() *stdLog.Logger {
	return stdLog.New(h, "", 0)
}

// Write implements the io.Writer interface.
// This is useful if you want to set this logger as a writer for the standard library log.
func (h Handler) Write(p []byte) (n int, err error) {
	n = len(p)
	if n > 0 && p[n-1] == '\n' {
		// Trim CR added by stdlog.
		p = p[0 : n-1]
	}

	var pcs [1]uintptr
	calldepth := 1
	runtime.Callers(calldepth+2, pcs[:])

	h.immediate = true
	err = h.Handle(
		slog.NewRecord(
			time.Now(),
			slog.LevelDebug,
			string(p),
			pcs[0],
			context.Background(),
		),
	)

	return
}

// circleBuf implements a very simple & naive circular buffer.
// users of circleBuf are responsible for concurrency safety.
type circleBuf struct {
	mu sync.Mutex // protects buf
	// +checklocks:mu
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
	c.buf = c.buf[:0]
}
