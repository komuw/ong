package log

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/akshayjshah/attest"
	ongErrors "github.com/komuw/ong/errors"
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

		{
			w := &bytes.Buffer{}
			maxMsgs := 3
			l := NewSlog(w, maxMsgs)

			infoMsg := "hello world"
			l(context.Background()).Info(infoMsg)
			l(context.Background()).Error("some-err", errors.New("bad"))

			attest.Subsequence(t, w.String(), "logID")
			attest.Subsequence(t, w.String(), "level")
			attest.Subsequence(t, w.String(), "source")
			attest.Subsequence(t, w.String(), slog.ErrorKey)
		}

		{
			w := &bytes.Buffer{}
			maxMsgs := 3
			l := NewSlog(w, maxMsgs)

			infoMsg := "hello world"
			l(context.Background()).Info(infoMsg)
			l(context.Background()).Error("some-ong-err", ongErrors.New("bad"))

			attest.Subsequence(t, w.String(), "logID")
			attest.Subsequence(t, w.String(), "stack")
		}
	})

	t.Run("logs are rotated", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := NewSlog(w, maxMsgs)

		for i := 0; i <= (maxMsgs + 4); i++ {
			infoMsg := "hello world" + " : " + fmt.Sprint(i)
			l(context.Background()).Info(infoMsg)
		}
		errMsg := "oops, Houston we got 99 problems."
		l(context.Background()).Error("somer-error", errors.New(errMsg))

		attest.False(t, strings.Contains(w.String(), "hello world : 1"))
		attest.False(t, strings.Contains(w.String(), "hello world : 2"))
		attest.False(t, strings.Contains(w.String(), "hello world : 5"))
		attest.Subsequence(t, w.String(), "hello world : 6")
		attest.Subsequence(t, w.String(), "hello world : 7")
		attest.Subsequence(t, w.String(), errMsg)
	})

	t.Run("WithContext does not invalidate buffer", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := NewSlog(w, maxMsgs)
		{
			for i := 0; i <= (maxMsgs); i++ {
				infoMsg := "hello world" + " : " + fmt.Sprint(i)
				l(context.Background()).Info(infoMsg)
			}
			attest.False(t, strings.Contains(w.String(), "hello world : 0"))
			attest.False(t, strings.Contains(w.String(), "hello world : 1"))
			attest.False(t, strings.Contains(w.String(), "hello world : 2"))
			attest.False(t, strings.Contains(w.String(), "hello world : 3"))
		}

		{
			xl := l(context.Background())
			l := xl.WithContext(context.Background())
			errMsg := "oops, Houston we got 99 problems."
			l.Error("some-error", errors.New(errMsg))

			attest.False(t, strings.Contains(w.String(), "hello world : 0"))
			attest.False(t, strings.Contains(w.String(), "hello world : 1"))
			attest.False(t, strings.Contains(w.String(), "hello world : 2"))
			attest.Subsequence(t, w.String(), "hello world : 3")
			attest.Subsequence(t, w.String(), errMsg)
		}
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 12
		l := NewSlog(w, maxMsgs)

		xl := l(context.Background())

		tokens := []string{
			"a", "aa", "aaa", "aaron", "ab", "abandoned", "abc", "aberdeen", "abilities", "ability", "able", "aboriginal", "abortion",
			"about", "above", "abraham", "abroad", "abs", "absence", "absent", "absolute", "absolutely", "absorption", "abstract",
			"abstracts", "abu", "abuse", "ac", "academic", "academics", "academy", "acc", "accent", "accept", "acceptable", "acceptance",
			"accepted", "accepting", "accepts", "access", "accessed", "accessibility", "accessible", "accessing", "accessories",
			"accessory", "accident", "accidents", "accommodate", "accommodation", "accommodations", "accompanied", "accompanying",
			"accomplish", "accomplished", "accordance", "according", "accordingly", "account", "accountability", "accounting", "accounts",
			"accreditation", "accredited", "accuracy", "accurate", "accurately", "accused", "acdbentity", "ace",
		}

		for _, tok := range tokens {
			go func(t string) {
				l(context.Background()).Info("one" + t)
				xl.Info("one" + t)
			}(tok)
		}

		for _, tok := range tokens {
			go func(t string) {
				l(context.Background()).Error("some-error", errors.New("bad"+t))
			}(tok)
		}

		for _, tok := range tokens {
			go func(t string) {
				l(context.Background()).Error("some-other-error", errors.New("bad-two"+t))
			}(tok)
		}

		wg := &sync.WaitGroup{}
		for _, tok := range tokens {
			wg.Add(1)
			go func(t string) {
				defer wg.Done()
				l(context.Background()).Info("four" + t)
				xl.Error("some-xl-error", errors.New("four"+t))
			}(tok)
		}
		wg.Wait()
	})
}
