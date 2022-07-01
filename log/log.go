// Package log implements a simple logging package
package log

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	stdLog "log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/komuw/ong/errors"
	"github.com/komuw/ong/id"
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
const CtxKey = logContextKeyType("Ong-logID")

type Logger struct {
	w          io.Writer
	cBuf       *circleBuf
	ctx        context.Context
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
) Logger {
	logID := GetId(ctx)
	ctx = context.WithValue(ctx, CtxKey, logID)
	if maxMsgs < 1 {
		maxMsgs = 10
	}
	return Logger{
		w:          w,
		cBuf:       newCirleBuf(maxMsgs),
		ctx:        ctx,
		addCallers: false,
		flds:       nil,
		immediate:  false,
	}
}

// WithCtx return a new logger, based on l, with the given ctx.
func (l Logger) WithCtx(ctx context.Context) Logger {
	logID := GetId(ctx)
	ctx = context.WithValue(ctx, CtxKey, logID)

	return Logger{
		w:          l.w,
		cBuf:       l.cBuf, // we do not invalidate buffer; `l.cBuf.buf = l.cBuf.buf[:0]`
		ctx:        ctx,
		addCallers: l.addCallers,
		flds:       l.flds,
		immediate:  l.immediate,
	}
}

// WithCaller return a new logger, based on l, that will include callers info in its output.
func (l Logger) WithCaller() Logger {
	return l.withcaller(true)
}

func (l Logger) withcaller(add bool) Logger {
	return Logger{
		w:          l.w,
		cBuf:       l.cBuf, // we do not invalidate buffer; `l.cBuf.buf = l.cBuf.buf[:0]`
		ctx:        l.ctx,
		addCallers: add,
		flds:       l.flds,
		immediate:  l.immediate,
	}
}

// WithFields return a new logger, based on l, that will include the given fields in all its output.
func (l Logger) WithFields(f F) Logger {
	return Logger{
		w:          l.w,
		cBuf:       l.cBuf, // we do not invalidate buffer; `l.cBuf.buf = l.cBuf.buf[:0]`
		ctx:        l.ctx,
		addCallers: l.addCallers,
		flds:       f,
		immediate:  l.immediate,
	}
}

// WithImmediate return a new logger, based on l, that will log immediately without buffering.
func (l Logger) WithImmediate() Logger {
	return Logger{
		w:          l.w,
		cBuf:       l.cBuf, // we do not invalidate buffer; `l.cBuf.buf = l.cBuf.buf[:0]`
		ctx:        l.ctx,
		addCallers: l.addCallers,
		flds:       l.flds,
		immediate:  true,
	}
}

// Info will log at the Info level.
func (l Logger) Info(f F) {
	l.log(infoL, f)
}

// Error will log at the Info level.
func (l Logger) Error(e error, fs ...F) {
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
// This is useful if you want to set this logger as a writer for the standard library log.
//
// usage:
//   l := log.New(ctx, os.Stdout, 100, true)
//   stdLogger := stdLog.New(l, "stdlib", stdLog.LstdFlags)
//   stdLogger.Println("hello world")
//
func (l Logger) Write(p []byte) (n int, err error) {
	n = len(p)
	if n > 0 && p[n-1] == '\n' {
		// Trim CR added by stdlog.
		p = p[0 : n-1]
	}
	// NB: we need to disable callers, otherwise this line is going to be listed as the caller..
	//     the caller is the line where `.Info` or `.Error` is called.
	l.withcaller(false).WithImmediate().Info(F{"message": string(p)})
	return
}

// StdLogger returns a logger from the Go standard library log package.
// That logger will use l as its output.
// usage:
//   l := log.New(ctx, os.Stdout, 100, true)
//   stdLogger := l.StdLogger()
//   stdLogger.Println("hey")
//
func (l Logger) StdLogger() *stdLog.Logger {
	l = l.WithImmediate().WithCaller()
	return stdLog.New(l, "", 0)
}

func (l Logger) log(lvl level, f F) {
	f["level"] = lvl
	f["timestamp"] = time.Now().UTC()
	f["logID"] = GetId(l.ctx)
	if l.addCallers {
		// the caller is the line where `.Info` or `.Error` is called.
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

func (l Logger) flush() {
	b := &bytes.Buffer{}
	encoder := json.NewEncoder(b)

	{
		l.cBuf.mu.Lock()
		for _, v := range l.cBuf.buf {
			if v == nil {
				continue
			}
			if err := encoder.Encode(v); err != nil {
				if os.Getenv("ONG_RUNNING_IN_TESTS") != "" {
					panic(err)
				}
				continue
			}
		}
		if _, err := l.w.Write(b.Bytes()); err != nil && os.Getenv("ONG_RUNNING_IN_TESTS") != "" {
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
	return id.New()
}

// circleBuf implements a very simple & naive circular buffer.
type circleBuf struct {
	mu      sync.Mutex // protects buf
	buf     []F
	maxSize int
}

func newCirleBuf(maxSize int) *circleBuf {
	if maxSize <= 0 {
		maxSize = 10
	}
	c := &circleBuf{
		buf:     make([]F, maxSize),
		maxSize: maxSize,
	}
	c.reset() // remove the nils from `make()`
	return c
}

func (c *circleBuf) store(f F) {
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

func (c *circleBuf) String() string {
	var s strings.Builder
	s.WriteString("circleBuf{\n")
	for _, v := range c.buf {
		s.WriteString(fmt.Sprintf("%v\n", v))
	}
	s.WriteString("\n}")

	return s.String()
}
