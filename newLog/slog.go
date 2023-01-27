package log

import (
	"context"
	"os"
	"sync"

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
type cHandler struct{ h slog.Handler }

func (s cHandler) Enabled(_ slog.Level) bool { return true /* support all logging levels*/ }
func (s cHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &cHandler{h: s.h.WithAttrs(attrs)}
}

func (s cHandler) WithGroup(name string) slog.Handler {
	return &cHandler{h: s.h.WithGroup(name)}
}

func (s cHandler) Handle(r slog.Record) (err error) {
	ctx := r.Context
	if ctx == nil {
		return s.h.Handle(r)
	}

	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return s.h.Handle(r)
	}

	{ // (a) adds TraceIds & spanIds to logs.
		//
		// TODO: (komuw) add stackTraces maybe.
		//
		sCtx := span.SpanContext()
		attrs := make([]slog.Attr, 0)
		if sCtx.HasTraceID() {
			attrs = append(attrs,
				slog.Attr{Key: "traceId", Value: slog.StringValue(sCtx.TraceID().String())},
			)
		}
		if sCtx.HasSpanID() {
			attrs = append(attrs,
				slog.Attr{Key: "spanId", Value: slog.StringValue(sCtx.SpanID().String())},
			)
		}
		if len(attrs) > 0 {
			r.AddAttrs(attrs...)
		}
	}

	{ // (b) adds logs to the active span as events.

		// code from: https://github.com/uptrace/opentelemetry-go-extra/tree/main/otellogrus
		// which is BSD 2-Clause license.

		attrs := make([]attribute.KeyValue, 0)

		logSeverityKey := attribute.Key("log.severity")
		logMessageKey := attribute.Key("log.message")
		attrs = append(attrs, logSeverityKey.String(r.Level.String()))
		attrs = append(attrs, logMessageKey.String(r.Message))

		// TODO: Obey the following rules form the slog documentation:
		//
		// Handle methods that produce output should observe the following rules:
		//   - If r.Time is the zero time, ignore the time.
		//   - If an Attr's key is the empty string, ignore the Attr.
		//
		r.Attrs(func(a slog.Attr) {
			attrs = append(attrs,
				attribute.KeyValue{
					Key:   attribute.Key(a.Key),
					Value: attribute.StringValue(a.Value.String()),
				},
			)
		})
		// todo: add caller info.

		span.AddEvent("log", trace.WithAttributes(attrs...))
		if r.Level >= slog.LevelError {
			span.SetStatus(codes.Error, r.Message)
		}
	}

	return s.h.Handle(r)
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
