// Package log implements a simple logging handler.
package log

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"

	ongErrors "github.com/komuw/ong/errors"
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
	if ctx == nil {
		ctx = context.Background()
	}
	id, _ := getId(ctx)
	return id
}

// getId gets a logId either from the provided context or auto-generated.
// It returns the logID and true if the id came from ctx else false
func getId(ctx context.Context) (string, bool) {
	if vCtx := ctx.Value(octx.LogCtxKey); vCtx != nil {
		if s, ok := vCtx.(string); ok {
			return s, true
		}
	}
	return id.New(), false
}

// New returns an [slog.Logger]
// The logger is backed by an [slog.Handler] that stores log messages into a [circular buffer].
// Those log messages are only flushed to the underlying io.Writer when a message with level >= [slog.LevelError] is logged.
// A unique logID is also added to the logs that acts as a correlation id of log events from diffrent components that
// neverthless are related.
//
// [circular buffer]: https://en.wikipedia.org/wiki/Circular_buffer
func New(ctx context.Context, w io.Writer, maxSize int) *slog.Logger {
	h := newHandler(ctx, w, maxSize)
	return slog.New(h)
}

// WithID returns a [slog.Logger] based on l, that includes a logID from ctx.
// If ctx does not contain a logID, one will be auto-generated.
func WithID(ctx context.Context, l *slog.Logger) *slog.Logger {
	if hdlr, okHandler := l.Handler().(*handler); okHandler {
		hdlr.mu.Lock()
		defer hdlr.mu.Unlock()

		id := hdlr.logID
		if id2, fromCtx := getId(ctx); fromCtx {
			id = id2
		}
		hdlr.logID = id
	}

	return l
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
	wrappedHandler slog.Handler

	// mu protects cBuf & logID
	// For why it is a pointer to mutex(which is unusual),
	// see: https://github.com/golang/go/commit/847d40d699832a1e054bc08c498548eff6a73ab6
	//      https://github.com/golang/example/blob/master/slog-handler-guide/README.md
	mu *sync.Mutex
	// +checklocks:mu
	cBuf *circleBuf
	// +checklocks:mu
	logID string
}

func newHandler(ctx context.Context, w io.Writer, maxSize int) slog.Handler {
	opts := &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.SourceKey {
				if t, ok := a.Value.Any().(*slog.Source); ok {
					// log the source in one line.
					return slog.String(a.Key, fmt.Sprintf("%s:%d", t.File, t.Line))
				}
			}
			return a
		},
	}

	return &handler{
		wrappedHandler: slog.NewJSONHandler(
			// os.Stderr is not buffered. Thus it will make a sycall for every write.
			// os.Stdout on the other hand is buffered.
			// https://eklitzke.org/stdout-buffering
			w,
			opts,
		),
		mu:    &sync.Mutex{},
		cBuf:  newCirleBuf(maxSize),
		logID: GetId(ctx),
	}
}

func (h *handler) Enabled(_ context.Context, l slog.Level) bool {
	return true
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h.mu.Lock()
	cBuf := h.cBuf
	id := h.logID
	h.mu.Unlock()
	return &handler{wrappedHandler: h.wrappedHandler.WithAttrs(attrs), mu: h.mu, cBuf: cBuf, logID: id}
}

func (h *handler) WithGroup(name string) slog.Handler {
	h.mu.Lock()
	cBuf := h.cBuf
	id := h.logID
	h.mu.Unlock()
	return &handler{wrappedHandler: h.wrappedHandler.WithGroup(name), mu: h.mu, cBuf: cBuf, logID: id}
}

func (h *handler) Handle(ctx context.Context, r slog.Record) error {
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

	{ // 1. save record.
		h.mu.Lock()
		h.cBuf.store(extendedLogRecord{r: r, logID: h.logID, ctx: ctx})
		h.mu.Unlock()
	}

	{ // 2. flush on error.
		if r.Level >= slog.LevelError {
			h.mu.Lock()
			defer h.mu.Unlock()

			var err error
			for _, v := range h.cBuf.buf {
				{ // 3. Add some required fields.

					// Convert time to UTC.
					// Note that we do not convert any other fields(that may be of type time.Time) into UTC.
					// If we ever need that functionality, we would do that in `r.Attrs()`
					if !v.r.Time.IsZero() {
						// According to the docs, If r.Time is the zero time, ignore the time.
						v.r.Time = v.r.Time.UTC()
					}

					newAttrs := []slog.Attr{}

					// Add logID
					theID := v.logID
					id2, fromCtx := getId(v.ctx)
					if fromCtx || (theID == "") {
						theID = id2
					}
					newAttrs = []slog.Attr{
						{Key: logIDFieldName, Value: slog.StringValue(theID)},
					}

					// Add stackTraces
					v.r.Attrs(func(a slog.Attr) bool {
						if e, ok := a.Value.Any().(error); ok {
							if stack := ongErrors.StackTrace(e); stack != "" {
								newAttrs = append(newAttrs, slog.Attr{Key: "stack", Value: slog.StringValue(stack)})
								return false // Stop iteration. This assumes that the log fields had only one error.
							}
						}
						return true
					})

					v.r.AddAttrs(newAttrs...)
				}

				{ // 4. flush to underlying handler.
					if e := h.wrappedHandler.Handle(v.ctx, v.r); e != nil {
						err = errors.Join([]error{err, e}...)
					}
				}
			}

			if err == nil {
				// Only reset if `h.Handler.Handle` succeded.
				// This is so that users do not lose valuable info that might be useful in debugging their app.
				h.cBuf.reset()
			}
			return err
		} else if r.Level == LevelImmediate {
			return h.wrappedHandler.Handle(ctx, r)
		}
	}

	return nil
}

// extendedLogRecord is similar to [slog.Record] except that
// it has been expanded to also include items that are specific to ong/log.
type extendedLogRecord struct {
	r     slog.Record
	logID string
	ctx   context.Context
}

// circleBuf implements a very simple & naive circular buffer.
// users of circleBuf are responsible for concurrency safety.
type circleBuf struct {
	buf     []extendedLogRecord
	maxSize int
}

func newCirleBuf(maxSize int) *circleBuf {
	if maxSize <= 0 {
		maxSize = 10
	}
	c := &circleBuf{
		buf:     make([]extendedLogRecord, maxSize),
		maxSize: maxSize,
	}
	c.reset() // remove the nils from `make()`
	return c
}

// store is a private api(thus needs no locking).
// It should only ever be called by `handler.Handle` which already takes a lock.
func (c *circleBuf) store(r extendedLogRecord) {
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
func (c *circleBuf) reset() {
	c.buf = c.buf[:0]
}
