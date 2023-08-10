// Package log implements a simple logging handler.
package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/komuw/ong/errors"
	"github.com/komuw/ong/id"
	"github.com/komuw/ong/internal/octx"
)

const (
	// ImmediateLevel is the severity which if a log event has, it is logged immediately without buffering.
	LevelImmediate = slog.Level(-6142973)

	// logIdFieldName is the name under which a logID will be logged as.
	logIDFieldName = "logID"
)

// GetId gets a logId either from the provided context or auto-generated.
func GetId(ctx context.Context) string {
	id, _ := getId(ctx)
	return id
}

// getId gets a logId either from the provided context or auto-generated.
// It returns the logID and true if the id came from ctx else false
func getId(ctx context.Context) (string, bool) {
	if ctx != nil {
		if vCtx := ctx.Value(octx.LogCtxKey); vCtx != nil {
			if s, ok := vCtx.(string); ok {
				return s, true
			}
		}
	}
	return id.New(), false
}

// New returns a function that returns an [slog.Logger]
// The logger is backed by a handler that stores log messages into a [circular buffer].
// Those log messages are only flushed to the underlying io.Writer when a message with level >= [slog.LevelError] is logged.
// A unique logID is also added to the logs that acts as a correlation id of log events from diffrent components that
// neverthless are related.
//
// [circular buffer]: https://en.wikipedia.org/wiki/Circular_buffer
func New(w io.Writer, maxMsgs int) func(ctx context.Context) *slog.Logger {
	opts := &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			v := a.Value
			if a.Key == "source" {
				if t, ok := v.Any().(*slog.Source); ok {
					// log the source in one line.
					return slog.String(a.Key, fmt.Sprintf("%s:%d", t.File, t.Line))
				}
			}
			return a
		},
	}
	jh := slog.NewJSONHandler(
		// os.Stderr is not buffered. Thus it will make a sycall for every write.
		// os.Stdout on the other hand is buffered.
		// https://eklitzke.org/stdout-buffering
		w,
		opts,
	)
	cbuf := newCirleBuf(maxMsgs)
	hdlr := handler{wrappedHandler: jh, cBuf: cbuf, logID: id.New()}
	l := slog.New(hdlr)

	return func(ctx context.Context) *slog.Logger {
		id := hdlr.logID
		if id2, ok := getId(ctx); ok {
			// if ctx did not contain a logId, do not use the generated one.
			id = id2
		}

		return l.With(logIDFieldName, id)
	}
}

// WithID returns a [slog.Logger] based on l, that includes a logID from ctx.
// If ctx does not contain a logID, one will be auto-generated.
func WithID(ctx context.Context, l *slog.Logger) *slog.Logger {
	id, fromCtx := getId(ctx)

	if hdlr, okHandler := l.Handler().(handler); okHandler {
		id2 := hdlr.logID
		if !fromCtx {
			id = id2
		}
	}

	return l.With(logIDFieldName, id)
}

// handler is an [slog.handler]
// It stores log messages into a [circular buffer]. Those log messages are only flushed to the underlying io.Writer when
// a message with level >= [slog.LevelError] is logged.
//
// [circular buffer]: https://en.wikipedia.org/wiki/Circular_buffer
type handler struct {
	// This handler is similar to python's memory handler.
	// https://github.com/python/cpython/blob/v3.11.1/Lib/logging/handlers.py#L1353-L1359
	//
	// from [slog.Handler] documentation:
	// Any of the Handler's methods may be called concurrently with itself or with other methods.
	// It is the responsibility of the Handler to manage this concurrency.
	// https://go-review.googlesource.com/c/exp/+/463255/2/slog/doc.go
	//
	// Also see: https://github.com/golang/example/blob/master/slog-handler-guide/README.md
	wrappedHandler *slog.JSONHandler // slog.Handler
	cBuf           *circleBuf
	logID          string
	attrs          []slog.Attr
}

func (h handler) Enabled(_ context.Context, _ slog.Level) bool {
	return true /* support all logging levels*/
}

func (h handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h.attrs = append(h.attrs, attrs...)
	return handler{wrappedHandler: h.wrappedHandler, cBuf: h.cBuf, logID: h.logID, attrs: h.attrs}
}

func (h handler) WithGroup(name string) slog.Handler {
	// return handler{wrappedHandler: h.wrappedHandler.WithGroup(name), cBuf: h.cBuf, logID: h.logID}
	// TODO:
	return handler{wrappedHandler: h.wrappedHandler, cBuf: h.cBuf, logID: h.logID, attrs: h.attrs}
}

func (h handler) Handle(ctx context.Context, r slog.Record) error {
	// Obey the following rules form the slog documentation:
	// Handle methods that produce OUTPUT should observe the following rules:
	//   - If r.Time is the zero time, ignore the time.
	//   - If r.PC is zero, ignore it.
	//   - Attr's values should be resolved.
	//   - If an Attr's key and value are both the zero value, ignore the Attr.
	//     This can be tested with attr.Equal(Attr{}).
	//   - If a group's key is empty, inline the group's Attrs.
	//   - If a group has no Attrs (even if it has a non-empty key), ignore it.
	// https://github.com/golang/go/blob/5c154986094bcc2fb28909cc5f01c9ba1dd9ddd4/src/log/slog/handler.go#L50-L59
	// Note that this handler does not produce output and hence the above rules do not apply.
	//
	// Also see: https://github.com/golang/example/blob/master/slog-handler-guide/README.md

	if ctx == nil {
		ctx = context.Background()
	}

	// Convert time to UTC.
	// Note that we do not convert any other fields(that may be of type time.Time) into UTC.
	// If we ever need that functionality, we would do that in `r.Attrs()`
	if r.Time.IsZero() {
		r.Time = time.Now().UTC()
	}
	r.Time = r.Time.UTC()

	newAttrs := h.attrs
	id := h.logID
	id2, fromCtx := getId(ctx)
	if fromCtx {
		id = id2
		newAttrs = slices.DeleteFunc(newAttrs, func(n slog.Attr) bool {
			// delete old logID
			return n.Key == logIDFieldName
		})
		newAttrs = append(newAttrs, slog.Attr{Key: logIDFieldName, Value: slog.StringValue(id)})
	}
	ctx = context.WithValue(ctx, octx.LogCtxKey, id)

	r.Attrs(func(a slog.Attr) bool {
		if e, ok := a.Value.Any().(error); ok {
			if stack := errors.StackTrace(e); stack != "" {
				newAttrs = append(newAttrs, slog.Attr{Key: "stack", Value: slog.StringValue(stack)})
			}
		}
		return true
	})
	if len(newAttrs) > 0 {
		r.AddAttrs(newAttrs...)
	}

	h.cBuf.mu.Lock()
	defer h.cBuf.mu.Unlock()

	// store record only after manipulating it.
	h.cBuf.store(r)

	var err error
	if r.Level >= slog.LevelError {
		for _, v := range h.cBuf.buf {
			err = h.wrappedHandler.Handle(ctx, v)
		}
		h.cBuf.reset()
	} else if r.Level == LevelImmediate {
		err = h.wrappedHandler.Handle(ctx, r)
	}

	return err
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

// store is a private api(thus needs no locking).
// It should only ever be called by `handler.Handle` which already takes a lock.
// +checklocksignore
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

// reset is a private api(thus needs no locking).
// It should only ever be called by `handler.Handle` which already takes a lock.
// +checklocksignore
func (c *circleBuf) reset() {
	c.buf = c.buf[:0]
}
