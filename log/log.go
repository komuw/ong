package log

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rs/xid"
)

type (
	level             uint8
	logContextKeyType string
	// F is the fields to use as a log message.
	F map[string]interface{}
)

const (
	infoL level = iota
	errorL
	disabledL // disabledL disables the logger.
)

var logCtxKey = logContextKeyType("logContextKey")

type logger struct {
	w      io.Writer
	cBuf   *circleBuf
	ctx    context.Context
	indent bool
}

func New(ctx context.Context, w io.Writer, maxMsgs int, indent bool) logger {
	logID := getLogId(ctx)
	ctx = context.WithValue(ctx, logCtxKey, logID)
	if maxMsgs < 1 {
		maxMsgs = 10
	}
	return logger{
		w:      w,
		cBuf:   newCirleBuf(maxMsgs),
		ctx:    ctx,
		indent: indent,
	}
}

func (l logger) WithCtx(ctx context.Context) logger {
	logID := getLogId(ctx)
	ctx = context.WithValue(ctx, logCtxKey, logID)

	cBuf := l.cBuf
	cBuf.buf = cBuf.buf[:0] // TODO: add tests to prove that m.cBuf.buf has not been invalidated.
	return logger{
		w:      l.w,
		cBuf:   cBuf,
		ctx:    ctx,
		indent: l.indent,
	}
}

func (l logger) log(lvl level, f F) {
	f["timestamp"] = time.Now().UTC()
	f["logID"] = getLogId(l.ctx)
	l.cBuf.store(f)

	if lvl >= errorL {
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

	// TODO: mutex lock read of m.cBuf.buf
	for _, v := range l.cBuf.buf {
		if v == nil {
			continue
		}
		if err := encoder.Encode(v); err != nil {
			// TODO: error in tests
			continue
		}
	}

	l.w.Write(b.Bytes())
	l.cBuf.reset()
}

// Info will log at the Info level.
func (l logger) Info(f F) {
	l.log(infoL, f)
}

// Error will log at the Info level.
func (l logger) Error(f F) {
	l.log(errorL, f)
}

func getLogId(ctx context.Context) string {
	if ctx != nil {
		if vCtx := ctx.Value(logCtxKey); vCtx != nil {
			if s, ok := vCtx.(string); ok {
				return s
			}
		}
	}
	return xid.New().String()
}

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
		buf:         make([]F, maxSize, maxSize),
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

func main() {
	my := New(context.Background(), os.Stdout, 3, true)
	my.Info(F{"one": "one", "ok": "okay"})
	my.Info(F{"two": "two"})
	my.Info(F{"three": "three"})
	my.Info(F{"four": "four"})
	my.Info(F{"five": "five"})
	my.Error(F{"err": "oops"})

}
