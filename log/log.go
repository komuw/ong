// Package log implements a simple logging package
package log

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/komuw/goweb/errors"
	"github.com/rs/xid"
	"golang.org/x/exp/maps"
)

// Most of the code here is insipired by(or taken from):
//   (a) https://github.com/rs/zerolog whose license(MIT) can be found here:      https://github.com/rs/zerolog/blob/v1.27.0/LICENSE
//   (b) https://github.com/sirupsen/logrus whose license(MIT) can be found here: https://github.com/sirupsen/logrus/blob/v1.8.1/LICENSE

type (
	level             string
	logContextKeyType string
	// F is the fields to use as a log message.
	F map[string]interface{}
)

const (
	infoL  level = "info"
	errorL level = "error"
)

// CtxKey is the name under which this library stores the http cookie, http header and context key for the logID.
const CtxKey = logContextKeyType("Goweb-logID")

type logger struct {
	w          io.Writer
	cBuf       *circleBuf
	ctx        context.Context
	indent     bool
	addCallers bool
	flds       F
	immediate  bool // log without buffering. important especially when using logger as an output for the stdlib logger.
}

// todo: add heartbeat in the future.

// New creates a new logger.
func New(
	ctx context.Context,
	w io.Writer,
	maxMsgs int,
	indent bool,
) logger {
	logID := GetId(ctx)
	ctx = context.WithValue(ctx, CtxKey, logID)
	if maxMsgs < 1 {
		maxMsgs = 10
	}
	return logger{
		w:          w,
		cBuf:       newCirleBuf(maxMsgs),
		ctx:        ctx,
		indent:     indent,
		addCallers: false,
		flds:       nil,
		immediate:  false,
	}
}

// WithCtx return a new logger, based on l, with the given ctx.
func (l logger) WithCtx(ctx context.Context) logger {
	logID := GetId(ctx)
	ctx = context.WithValue(ctx, CtxKey, logID)

	return logger{
		w:          l.w,
		cBuf:       l.cBuf, // we do not invalidate buffer; `l.cBuf.buf = l.cBuf.buf[:0]`
		ctx:        ctx,
		indent:     l.indent,
		addCallers: l.addCallers,
		flds:       l.flds,
		immediate:  l.immediate,
	}
}

// WithCaller return a new logger, based on l, that will include callers info in its output.
func (l logger) WithCaller() logger {
	return logger{
		w:          l.w,
		cBuf:       l.cBuf, // we do not invalidate buffer; `l.cBuf.buf = l.cBuf.buf[:0]`
		ctx:        l.ctx,
		indent:     l.indent,
		addCallers: true,
		flds:       l.flds,
		immediate:  l.immediate,
	}
}

// WithFields return a new logger, based on l, that will include the given fields in all its output.
func (l logger) WithFields(f F) logger {
	return logger{
		w:          l.w,
		cBuf:       l.cBuf, // we do not invalidate buffer; `l.cBuf.buf = l.cBuf.buf[:0]`
		ctx:        l.ctx,
		indent:     l.indent,
		addCallers: l.addCallers,
		flds:       f,
		immediate:  l.immediate,
	}
}

// WithImmediate return a new logger, based on l, that will log immediately without buffering if immediate is true.
func (l logger) WithImmediate(immediate bool) logger {
	return logger{
		w:          l.w,
		cBuf:       l.cBuf, // we do not invalidate buffer; `l.cBuf.buf = l.cBuf.buf[:0]`
		ctx:        l.ctx,
		indent:     l.indent,
		addCallers: l.addCallers,
		flds:       l.flds,
		immediate:  immediate,
	}
}

// Info will log at the Info level.
func (l logger) Info(f F) {
	l.log(infoL, f)
}

// Error will log at the Info level.
func (l logger) Error(e error, fs ...F) {
	dst := F{}
	if e != nil {
		dst = F{"err": e.Error()}
		if stack := errors.StackTrace(e); stack != "" {
			dst["stack"] = stack
		}
	}

	for _, f := range fs {
		for k, v := range f {
			dst[k] = v
		}
	}

	l.log(errorL, dst)
}

// Write implements the io.Writer interface.
// This is useful to set as a writer for the standard library log.
//
// usage:
//   l := log.New(ctx, os.Stdout, 100, true).WithImmediate(true)
//   sl := stdLog.New(l, "stdlib", stdLog.LstdFlags)
//   sl.Println("hello world")
//
func (l logger) Write(p []byte) (n int, err error) {
	n = len(p)
	if n > 0 && p[n-1] == '\n' {
		// Trim CR added by stdlog.
		p = p[0 : n-1]
	}
	l.WithCaller().Info(F{"message": string(p)})
	return
}

func (l logger) log(lvl level, f F) {
	f["level"] = lvl
	f["timestamp"] = time.Now().UTC()
	f["logID"] = GetId(l.ctx)
	if l.addCallers {
		if _, file, line, ok := runtime.Caller(2); ok {
			f["line"] = fmt.Sprintf("%s:%d", file, line)
		}
	}

	if l.flds != nil {
		// Copy(dst, src)
		maps.Copy(f, l.flds) // keys in dst(`f`) that are also in l.flds, are overwritten.
	}

	l.cBuf.store(f)

	if lvl == errorL || l.immediate {
		// flush
		l.flush()
	}
}

func (l logger) flush() {
	b := &bytes.Buffer{}
	encoder := json.NewEncoder(b)
	if l.indent {
		encoder.SetIndent("", "  ")
	}

	{
		l.cBuf.mu.Lock()
		for _, v := range l.cBuf.buf {
			if v == nil {
				continue
			}
			if err := encoder.Encode(v); err != nil {
				if os.Getenv("GOWEB_RUNNING_IN_TESTS") != "" {
					panic(err)
				}
				continue
			}
		}
		if _, err := l.w.Write(b.Bytes()); err != nil && os.Getenv("GOWEB_RUNNING_IN_TESTS") != "" {
			panic(err)
		}
		l.cBuf.mu.Unlock()
	}

	l.cBuf.reset()
}

// GetId returns a logID which is fetched either from the provided context or auto-generated.
func GetId(ctx context.Context) string {
	if ctx != nil {
		if vCtx := ctx.Value(CtxKey); vCtx != nil {
			if s, ok := vCtx.(string); ok {
				return s
			}
		}
	}
	return xid.New().String()
}

// circleBuf implements a very simple & naive circular buffer.
type circleBuf struct {
	mu          sync.Mutex // protects buf
	buf         []F
	maxSize     int
	currentSize int
}

func newCirleBuf(maxSize int) *circleBuf {
	if maxSize <= 0 {
		maxSize = 10
	}
	return &circleBuf{
		buf:         make([]F, maxSize),
		maxSize:     maxSize,
		currentSize: 0,
	}
}

func (c *circleBuf) String() string {
	var s strings.Builder
	s.WriteString("circleBuf{\n")
	for _, v := range c.buf {
		s.WriteString(fmt.Sprintf("%v\n", v))
	}
	s.WriteString("\n}")

	return s.String()
}

func (c *circleBuf) store(f F) {
	c.mu.Lock()
	defer c.mu.Unlock()

	availableSpace := c.maxSize - c.currentSize
	if availableSpace <= 0 {
		// clear space.
		upto := c.maxSize / 4
		c.buf = c.buf[:upto]
		c.currentSize = c.currentSize / 4
	}

	c.buf = append(c.buf, f)
	c.currentSize = c.currentSize + 1
}

func (c *circleBuf) reset() {
	c.mu.Lock()
	c.buf = c.buf[:0]
	c.mu.Unlock()
}
