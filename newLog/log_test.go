package log

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/akshayjshah/attest"
	"golang.org/x/exp/slog"
)

func TestCircleBuf(t *testing.T) {
	t.Parallel()

	t.Run("it stores", func(t *testing.T) {
		t.Parallel()

		maxSize := 4
		c := newCirleBuf(maxSize)

		c.store(slog.Record{Message: "one"})
		c.store(slog.Record{Message: "two"})

		attest.Equal(t, c.buf[0].Message, "one")

		attest.Equal(t, c.buf[1].Message, "two")

		attest.Equal(t, len(c.buf), 2)
		attest.Equal(t, cap(c.buf), 4)
	})

	t.Run("does not exceed maxsize", func(t *testing.T) {
		t.Parallel()

		maxSize := 8
		c := newCirleBuf(maxSize)
		for i := 0; i <= (13 * maxSize); i++ {
			x := fmt.Sprint(i)
			c.store(slog.Record{Message: x})

			attest.True(t, len(c.buf) <= maxSize)
			attest.True(t, cap(c.buf) <= maxSize)
		}
		attest.True(t, len(c.buf) <= maxSize)
		attest.True(t, cap(c.buf) <= maxSize)
	})

	t.Run("clears oldest first", func(t *testing.T) {
		t.Parallel()

		maxSize := 5
		c := newCirleBuf(maxSize)
		for i := 0; i <= (6 * maxSize); i++ {
			x := fmt.Sprint(i)
			c.store(slog.Record{Message: x})
			attest.True(t, len(c.buf) <= maxSize)
			attest.True(t, cap(c.buf) <= maxSize)
		}
		attest.True(t, len(c.buf) <= maxSize)
		attest.True(t, cap(c.buf) <= maxSize)

		attest.Equal(t, c.buf[1].Message, "29")
		attest.Equal(t, c.buf[2].Message, "30")
	})

	t.Run("reset", func(t *testing.T) {
		t.Parallel()

		maxSize := 80
		c := newCirleBuf(maxSize)
		for i := 0; i <= (13 * maxSize); i++ {
			x := fmt.Sprint(i)
			c.store(slog.Record{Message: x})
			attest.True(t, len(c.buf) <= maxSize)
			attest.True(t, cap(c.buf) <= maxSize)
		}
		attest.True(t, len(c.buf) <= maxSize)
		attest.True(t, cap(c.buf) <= maxSize)

		c.reset()

		attest.Equal(t, len(c.buf), 0)
		attest.Equal(t, cap(c.buf), maxSize)
	})
}

type syncBuffer struct {
	mu sync.Mutex
	b  *bytes.Buffer
}

func newBuf() *syncBuffer {
	return &syncBuffer{b: &bytes.Buffer{}}
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func (s *syncBuffer) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

// TODO: add test for ong/errors
func TestLogger(t *testing.T) {
	t.Parallel()

	t.Run("info level does not do anything", func(t *testing.T) {
		t.Parallel()

		w := newBuf()
		maxMsgs := 3
		l := NewSlog(w, maxMsgs)
		l(context.Background()).Info("hey", "one", "one")

		attest.Zero(t, w.String())
	})

	t.Run("error logs immediately", func(t *testing.T) {
		t.Parallel()

		w := newBuf()
		maxMsgs := 3
		l := NewSlog(w, maxMsgs)
		msg := "oops, Houston we got 99 problems."
		l(context.Background()).Error(msg, errors.New(msg))

		attest.Subsequence(t, w.String(), msg)
	})

	t.Run("info logs are flushed on error", func(t *testing.T) {
		t.Parallel()

		w := newBuf()
		maxMsgs := 3
		l := NewSlog(w, maxMsgs)

		infoMsg := "hello world"
		l(context.Background()).Info(infoMsg, "what", "ok", "passwd", "ak&dHyS>47K")
		errMsg := "oops, Houston we got 99 problems."
		l(context.Background()).Error("some-error", errors.New(errMsg))

		attest.Subsequence(t, w.String(), infoMsg)
		attest.Subsequence(t, w.String(), errMsg)

		// TODO: see what proposals of slog say about my question regarding this.
		// https://github.com/golang/go/issues/56345#issuecomment-1407491552
		//
		// special characters are not quoted.
		// attest.Subsequence(t, w.String(), "&")
		// attest.Subsequence(t, w.String(), ">")
	})

	t.Run("neccesary fields added", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := NewSlog(w, maxMsgs)

		// TODO: test that ong/errors adds StackTrace.
		{
			infoMsg := "hello world"
			l(context.Background()).Info(infoMsg)
			l(context.Background()).Error("some-err", errors.New("bad"))

			attest.Subsequence(t, w.String(), "logID")
			attest.Subsequence(t, w.String(), "level")
			attest.Subsequence(t, w.String(), "source")
			attest.Subsequence(t, w.String(), slog.ErrorKey)
			// attest.False(t, strings.Contains(w.String(), "line")) // line not added
		}

		// {
		// 	l = l.WithCaller()
		// 	l.Info(F{"name": "john"})
		// 	errMsg := "kimeumana"
		// 	l.Error(errors.New(errMsg))

		// 	id := l.logId
		// 	attest.NotZero(t, id)
		// 	attest.Subsequence(t, w.String(), id)
		// 	attest.Subsequence(t, w.String(), "level")
		// 	attest.Subsequence(t, w.String(), "stack")
		// 	attest.Subsequence(t, w.String(), "err")
		// 	attest.Subsequence(t, w.String(), "line") // line added
		// }
	})

	// t.Run("logs are rotated", func(t *testing.T) {
	// 	t.Parallel()

	// 	w := &bytes.Buffer{}
	// 	maxMsgs := 3
	// 	l := New(w, maxMsgs)

	// 	for i := 0; i <= (maxMsgs + 4); i++ {
	// 		infoMsg := "hello world" + " : " + fmt.Sprint(i)
	// 		l.Info(F{"what": infoMsg})
	// 	}
	// 	errMsg := "oops, Houston we got 99 problems."
	// 	l.Error(errors.New(errMsg))

	// 	attest.False(t, strings.Contains(w.String(), "hello world : 1"))
	// 	attest.False(t, strings.Contains(w.String(), "hello world : 2"))
	// 	attest.False(t, strings.Contains(w.String(), "hello world : 5"))
	// 	attest.Subsequence(t, w.String(), "hello world : 6")
	// 	attest.Subsequence(t, w.String(), "hello world : 7")
	// 	attest.Subsequence(t, w.String(), errMsg)
	// })

	// t.Run("various ways of calling l.Error", func(t *testing.T) {
	// 	t.Parallel()

	// 	w := &bytes.Buffer{}
	// 	maxMsgs := 3
	// 	l := New(w, maxMsgs)
	// 	msg := "some-error"
	// 	err := errors.New(msg)

	// 	l.Error(err)
	// 	l.Error(err, F{"one": "two"})
	// 	l.Error(err, F{"three": "four"}, F{"five": "six"})
	// 	l.Error(err, nil)
	// 	l.Error(nil)
	// 	l.Error(nil, F{"seven": "eight"})

	// 	attest.Subsequence(t, w.String(), msg)
	// 	for _, v := range []string{"one", "two", "three", "four", "five", "six", "seven", "eight"} {
	// 		attest.Subsequence(t, w.String(), v, attest.Sprintf("`%s` not found", v))
	// 	}
	// })

	// t.Run("WithCtx does not invalidate buffer", func(t *testing.T) {
	// 	t.Parallel()

	// 	w := &bytes.Buffer{}
	// 	maxMsgs := 3
	// 	l := New(w, maxMsgs)
	// 	{
	// 		for i := 0; i <= (maxMsgs); i++ {
	// 			infoMsg := "hello world" + " : " + fmt.Sprint(i)
	// 			l.Info(F{"what": infoMsg})
	// 		}
	// 		attest.False(t, strings.Contains(w.String(), "hello world : 0"))
	// 		attest.False(t, strings.Contains(w.String(), "hello world : 1"))
	// 		attest.False(t, strings.Contains(w.String(), "hello world : 2"))
	// 		attest.False(t, strings.Contains(w.String(), "hello world : 3"))
	// 	}

	// 	{
	// 		l = l.WithCtx(context.Background())
	// 		errMsg := "oops, Houston we got 99 problems."
	// 		l.Error(errors.New(errMsg))

	// 		attest.False(t, strings.Contains(w.String(), "hello world : 0"))
	// 		attest.False(t, strings.Contains(w.String(), "hello world : 1"))
	// 		attest.False(t, strings.Contains(w.String(), "hello world : 2"))
	// 		attest.Subsequence(t, w.String(), "hello world : 3")
	// 		attest.Subsequence(t, w.String(), errMsg)
	// 	}
	// })

	// t.Run("WithCaller does not invalidate buffer", func(t *testing.T) {
	// 	t.Parallel()

	// 	w := &bytes.Buffer{}
	// 	maxMsgs := 3
	// 	l := New(w, maxMsgs)
	// 	{
	// 		for i := 0; i <= (maxMsgs); i++ {
	// 			infoMsg := "hello world" + " : " + fmt.Sprint(i)
	// 			l.Info(F{"what": infoMsg})
	// 		}
	// 		attest.False(t, strings.Contains(w.String(), "hello world : 0"))
	// 		attest.False(t, strings.Contains(w.String(), "hello world : 1"))
	// 		attest.False(t, strings.Contains(w.String(), "hello world : 2"))
	// 		attest.False(t, strings.Contains(w.String(), "hello world : 3"))
	// 	}

	// 	{
	// 		l = l.WithCaller()
	// 		errMsg := "oops, Houston we got 99 problems."
	// 		l.Error(errors.New(errMsg))

	// 		attest.False(t, strings.Contains(w.String(), "hello world : 0"))
	// 		attest.False(t, strings.Contains(w.String(), "hello world : 1"))
	// 		attest.False(t, strings.Contains(w.String(), "hello world : 2"))
	// 		attest.Subsequence(t, w.String(), "hello world : 3")
	// 		attest.Subsequence(t, w.String(), errMsg)
	// 	}
	// })

	// t.Run("WithFields", func(t *testing.T) {
	// 	t.Parallel()

	// 	w := &bytes.Buffer{}
	// 	maxMsgs := 3
	// 	l := New(w, maxMsgs)
	// 	flds := F{"version": "v0.1.2", "env": "prod", "service": "web-commerce"}
	// 	l = l.WithFields(flds)

	// 	msg := "hello"
	// 	l.Info(F{"msg": msg})
	// 	errMsg := "oops, Houston we got 99 problems."
	// 	l.Error(errors.New(errMsg))

	// 	for _, v := range []string{
	// 		"version",
	// 		"v0.1.2",
	// 		"web-commerce",
	// 		msg,
	// 		errMsg,
	// 	} {
	// 		attest.Subsequence(t, w.String(), v)
	// 	}
	// 	attest.Equal(t, l.flds, flds)

	// 	newFlds := F{"okay": "yes", "country": "Norway"}
	// 	l = l.WithFields(newFlds)
	// 	newErrMsg := "new error"
	// 	l.Error(errors.New(newErrMsg))
	// 	// asserts that the `l.flds` maps does not grow without bound.
	// 	attest.Equal(t, l.flds, newFlds)
	// 	for _, v := range []string{
	// 		"okay",
	// 		"yes",
	// 		"Norway",
	// 		msg,
	// 		newErrMsg,
	// 	} {
	// 		attest.Subsequence(t, w.String(), v)
	// 	}
	// })

	// t.Run("WithImmediate logs immediately", func(t *testing.T) {
	// 	t.Parallel()

	// 	w := &bytes.Buffer{}
	// 	msg := "hello world"
	// 	l := New(w, 2).WithImmediate()
	// 	l.Info(F{"msg": msg})

	// 	attest.Subsequence(t, w.String(), msg)
	// })

	// t.Run("interop with stdlibLog", func(t *testing.T) {
	// 	t.Parallel()

	// 	w := &bytes.Buffer{}
	// 	msg := "hello world"
	// 	l := New(w, 2)
	// 	stdLogger := stdLog.New(l, "stdlib", stdLog.Lshortfile)
	// 	stdLogger.Println(msg)

	// 	attest.Subsequence(t, w.String(), msg)
	// })

	// t.Run("get stdlibLog", func(t *testing.T) {
	// 	t.Parallel()

	// 	w := &bytes.Buffer{}
	// 	msg := "hey what up?"
	// 	l := New(w, 2)
	// 	stdLogger := l.StdLogger()
	// 	stdLogger.Println(msg)
	// 	attest.Subsequence(t, w.String(), msg)
	// })

	// t.Run("WithCaller uses correct line", func(t *testing.T) {
	// 	t.Parallel()

	// 	{
	// 		w := &bytes.Buffer{}
	// 		msg := "hey what up?"
	// 		l := New(w, 2)
	// 		l.WithCaller().WithImmediate().Info(F{"msg": msg})
	// 		attest.Subsequence(t, w.String(), msg)
	// 		attest.Subsequence(t, w.String(), "ong/log/log_test.go:374")
	// 	}

	// 	{
	// 		// for stdlib we disable caller info, since it would otherwise
	// 		// point to `ong/log/log.go` as the caller.
	// 		w := &bytes.Buffer{}
	// 		msg := "hey what up?"
	// 		l := New(w, 2)
	// 		l.WithCaller().StdLogger().Println(msg)
	// 		attest.Subsequence(t, w.String(), msg)
	// 		attest.False(t, strings.Contains(w.String(), "ong/log/log_test.go"))
	// 	}

	// 	{
	// 		w := &bytes.Buffer{}
	// 		msg := "hey what up?"
	// 		l := New(w, 2).WithCaller()
	// 		stdLogger := stdLog.New(l, "stdlib", 0)
	// 		stdLogger.Println(msg)
	// 		attest.Subsequence(t, w.String(), msg)
	// 		attest.False(t, strings.Contains(w.String(), "ong/log/log_test.go"))
	// 	}
	// })

	// t.Run("concurrency safe", func(t *testing.T) {
	// 	t.Parallel()

	// 	w := &bytes.Buffer{}
	// 	maxMsgs := 3
	// 	l := New(w, maxMsgs)

	// 	tokens := []string{
	// 		"a", "aa", "aaa", "aaron", "ab", "abandoned", "abc", "aberdeen", "abilities", "ability", "able", "aboriginal", "abortion",
	// 		"about", "above", "abraham", "abroad", "abs", "absence", "absent", "absolute", "absolutely", "absorption", "abstract",
	// 		"abstracts", "abu", "abuse", "ac", "academic", "academics", "academy", "acc", "accent", "accept", "acceptable", "acceptance",
	// 		"accepted", "accepting", "accepts", "access", "accessed", "accessibility", "accessible", "accessing", "accessories",
	// 		"accessory", "accident", "accidents", "accommodate", "accommodation", "accommodations", "accompanied", "accompanying",
	// 		"accomplish", "accomplished", "accordance", "according", "accordingly", "account", "accountability", "accounting", "accounts",
	// 		"accreditation", "accredited", "accuracy", "accurate", "accurately", "accused", "acdbentity", "ace",
	// 	}

	// 	for _, tok := range tokens {
	// 		go func(t string) {
	// 			l.Info(F{"one": "one" + t})
	// 		}(tok)
	// 	}

	// 	for _, tok := range tokens {
	// 		go func(t string) {
	// 			l.Error(errors.New("bad" + t))
	// 		}(tok)
	// 	}

	// 	for _, tok := range tokens {
	// 		go func(t string) {
	// 			l.Error(errors.New("bad-two" + t))
	// 		}(tok)
	// 	}

	// 	wg := &sync.WaitGroup{}
	// 	for _, tok := range tokens {
	// 		wg.Add(1)
	// 		go func(t string) {
	// 			defer wg.Done()
	// 			l.Info(F{"four": "four" + t})
	// 		}(tok)
	// 	}
	// 	wg.Wait()
	// })
}
