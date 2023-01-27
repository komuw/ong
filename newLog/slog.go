package log

import (
	"context"
	"os"
	"sync"

	"github.com/komuw/ong/errors"
	"github.com/komuw/ong/id"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/exp/slog"
)

var (
	onceSlog   sync.Once
	slogLogger *slog.Logger
)

// usage:
//
//	ctx, span := tracer.Start(ctx, "myFuncName")
//	l := NewSlog(ctx)
//	l.Info("hello world")
func NewSlog(ctx context.Context) *slog.Logger {
	onceSlog.Do(func() {
		opts := slog.HandlerOptions{
			AddSource: true,
			Level:     slog.LevelDebug,
		}
		jh := opts.NewJSONHandler(os.Stdout)

		h := cHandler{h: jh}
		l := slog.New(h)
		slogLogger = l
	})

	return slogLogger.WithContext(ctx)
}

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
func (s cHandler) Handle(r slog.Record) (err error) {

	// TODO: make sure time is in UTC.
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

	if r.Level >= slog.LevelError {
		s.cBuf.mu.Lock()
		for _, v := range s.cBuf.buf {
			return s.h.Handle(r)
		}
		s.cBuf.mu.Unlock()

	}

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
	buf     []slog.Attr
	maxSize int
}

func newCirleBuf(maxSize int) *circleBuf {
	if maxSize <= 0 {
		maxSize = 10
	}
	c := &circleBuf{
		buf:     make([]slog.Attr, maxSize),
		maxSize: maxSize,
	}
	c.reset() // remove the nils from `make()`
	return c
}

func (c *circleBuf) store(f slog.Attr) {
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

	c.buf = append(c.buf, f)
}

func (c *circleBuf) reset() {
	c.mu.Lock()
	c.buf = c.buf[:0]
	c.mu.Unlock()
}
