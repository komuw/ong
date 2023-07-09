package log

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	mathRand "math/rand"
	"strings"
	"sync"
	"testing"

	ongErrors "github.com/komuw/ong/errors"
	"github.com/komuw/ong/internal/octx"

	"go.akshayshah.org/attest"
	"go.uber.org/goleak"
	"golang.org/x/exp/slog"
)

func TestMain(m *testing.M) {
	// call flag.Parse() here if TestMain uses flags
	goleak.VerifyTestMain(m)
}

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

func TestLogger(t *testing.T) {
	t.Parallel()

	t.Run("info level does not do anything", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(w, maxMsgs)
		l(context.Background()).Info("hey", "one", "one")

		attest.Zero(t, w.String())
	})

	t.Run("error logs immediately", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(w, maxMsgs)
		msg := "oops, Houston we got 99 problems."
		l(context.Background()).Error(msg, errors.New(msg))

		attest.Subsequence(t, w.String(), msg)

		{
			// it can log nils.
			l(context.Background()).Info(msg, "someValue", nil)
			l(context.Background()).Error(msg, nil)
		}
	})

	t.Run("info logs are flushed on error", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(w, maxMsgs)

		logger := l(context.Background())
		infoMsg := "hello world"
		logger.Info(infoMsg, "what", "ok", "passwd", "ak&dHyS>47K")
		errMsg := "oops, Houston we got 99 problems."
		logger.Error("some-error", errors.New(errMsg))

		hdlr, ok := logger.Handler().(handler)
		attest.True(t, ok)
		logID := hdlr.logID

		attest.Subsequence(t, w.String(), infoMsg)
		attest.Subsequence(t, w.String(), errMsg)
		attest.Subsequence(t, w.String(), logID)

		// special characters are not quoted.
		attest.Subsequence(t, w.String(), "&")
		attest.Subsequence(t, w.String(), ">")
	})

	t.Run("neccesary fields added", func(t *testing.T) {
		t.Parallel()

		{
			w := &bytes.Buffer{}
			maxMsgs := 3
			l := New(w, maxMsgs)

			infoMsg := "hello world"
			l(context.Background()).Info(infoMsg)
			l(context.Background()).Error("some-err", errors.New("this-ting-is-bad"))

			attest.Subsequence(t, w.String(), logIDFieldName)
			attest.Subsequence(t, w.String(), "level")
			attest.Subsequence(t, w.String(), "source")
			attest.Subsequence(t, w.String(), "this-ting-is-bad")
		}

		{
			w := &bytes.Buffer{}
			maxMsgs := 3
			l := New(w, maxMsgs)

			infoMsg := "hello world"
			l(context.Background()).Info(infoMsg)
			l(context.Background()).Error("some-ong-err", ongErrors.New("bad"))

			attest.Subsequence(t, w.String(), logIDFieldName)
			attest.Subsequence(t, w.String(), "stack")
			attest.Subsequence(t, w.String(), "log/log_test.go:184") // stacktrace added.
		}
	})

	t.Run("logs are rotated", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(w, maxMsgs)

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

	t.Run("id reused across contexts", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3

		l1 := New(w, maxMsgs)(context.Background())
		hdlr, ok := l1.Handler().(handler)
		attest.True(t, ok)
		logid1 := hdlr.logID
		l1.Error("hey1", errors.New("cool1"))

		attest.Subsequence(t, w.String(), "hey1")
		attest.Subsequence(t, w.String(), logid1)
		h, ok := l1.Handler().(handler)
		attest.True(t, ok)
		attest.Equal(t, len(h.cBuf.buf), 0)

		w.Reset() // clear buffer.

		l2 := l1.With("category", "load_balancers")
		l2.Error("hey2", errors.New("cool2"))
		attest.Subsequence(t, w.String(), "hey2")
		attest.False(t, strings.Contains(w.String(), "hey1")) // hey1 is not loggged here.
		attest.Subsequence(t, w.String(), logid1)
	})

	t.Run(
		// See: https://github.com/komuw/ong/issues/316
		"logID not duplicated",
		func(t *testing.T) {
			t.Parallel()

			w := &bytes.Buffer{}
			maxMsgs := 3

			msg := "hey"
			l := New(w, maxMsgs)(context.Background())
			l.Error(msg)

			attest.Equal(t, strings.Count(w.String(), logIDFieldName), 1)
			attest.Equal(t, strings.Count(w.String(), "msg"), 1)
			attest.Equal(t, strings.Count(w.String(), msg), 1)
			attest.Equal(t, strings.Count(w.String(), "time"), 1)
			attest.Equal(t, strings.Count(w.String(), "level"), 1)
		},
	)

	t.Run("New context does not invalidate buffer", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(w, maxMsgs)
		{
			logger1 := l(context.Background())
			for i := 0; i <= (maxMsgs); i++ {
				infoMsg := "hello world" + " : " + fmt.Sprint(i)
				logger1.Info(infoMsg)
			}
			attest.False(t, strings.Contains(w.String(), "hello world : 0"))
			attest.False(t, strings.Contains(w.String(), "hello world : 1"))
			attest.False(t, strings.Contains(w.String(), "hello world : 2"))
			attest.False(t, strings.Contains(w.String(), "hello world : 3"))
		}

		{
			logger2 := l(context.Background()).With("name", "Kim")
			errMsg := "oops, Houston we got 99 problems."
			logger2.Error("some-error", errors.New(errMsg))

			attest.False(t, strings.Contains(w.String(), "hello world : 0"))
			attest.False(t, strings.Contains(w.String(), "hello world : 1"))
			attest.False(t, strings.Contains(w.String(), "hello world : 2"))
			attest.Subsequence(t, w.String(), "hello world : 3")
			attest.Subsequence(t, w.String(), errMsg)
		}
	})

	t.Run("ctx methods", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		w := &bytes.Buffer{}
		msg := "hey what up?"
		l := New(w, 3)(ctx)

		l.InfoCtx(ctx, msg)
		l.ErrorCtx(ctx, "hey1", "err1", errors.New("badTingOne"))
		attest.Subsequence(t, w.String(), msg)
		attest.Equal(t,
			strings.Count(w.String(), logIDFieldName),
			// one for the info log the other for the error log
			2,
		)

		newId := "NEW-id-adh4e92427dajd"
		ctx = context.WithValue(ctx, octx.LogCtxKey, newId)
		l.ErrorCtx(ctx, "hey2", "err2", errors.New("badTingTwo"))
		fmt.Println("\n\t ", w.String(), "\n.")
		attest.Subsequence(t, w.String(), newId)
		attest.Equal(t,
			strings.Count(w.String(), logIDFieldName),
			// one for the info log the other for the badTingOne error log & one for badTingTwo error log.
			3,
		)
	})

	t.Run("stdlibLog", func(t *testing.T) {
		t.Parallel()

		{ // normal loglevels go through circular buffer.
			w := &bytes.Buffer{}
			msg := "hey what up?"
			l := New(w, 2)(context.Background())

			stdLogger := slog.NewLogLogger(l.Handler(), slog.LevelInfo)
			stdLogger.Println(msg)
			attest.Zero(t, w.String())
		}

		{ // `LevelImmediate` loglevel is logged ASAP.
			w := &bytes.Buffer{}
			msg := "hey what up?"
			l := New(w, 2)(context.Background())

			stdLogger := slog.NewLogLogger(l.Handler(), LevelImmediate)
			stdLogger.Println(msg)
			attest.Subsequence(t, w.String(), msg)
			attest.Subsequence(t, w.String(), `log/log_test.go`)
			attest.Subsequence(t, w.String(), `line":309`)
			attest.True(t, LevelImmediate < 0) // otherwise it will trigger `log.handler` to flush all logs, which we dont want.
		}
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 12
		l := New(w, maxMsgs)

		xl := l(context.Background())

		tokens := []string{
			"a", "aa", "aaa", "aaron", "ab", "abandoned", "abc", "aberdeen", "abilities", "ability", "able", "aboriginal", "abortion",
			"about", "above", "abraham", "abroad", "abs", "absence", "absent", "absolute", "absolutely", "absorption", "abstract",
			"accessory", "accident", "accidents", "accommodate", "accommodation", "accommodations", "accompanied", "accompanying",
			"accomplish", "accomplished", "accordance", "according", "accordingly", "account", "accountability", "accounting", "accounts",
			"about", "above", "abraham", "abroad", "abs", "absence", "absent", "absolute", "absolutely", "absorption", "abstract",
			"abstracts", "abu", "abuse", "ac", "academic", "academics", "academy", "acc", "accent", "accept", "acceptable", "acceptance",
			"abstracts", "abu", "abuse", "ac", "academic", "academics", "academy", "acc", "accent", "accept", "acceptable", "acceptance",
			"accepted", "accepting", "accepts", "access", "accessed", "accessibility", "accessible", "accessing", "accessories",
			"accessory", "accident", "accidents", "accommodate", "accommodation", "accommodations", "accompanied", "accompanying",
			"abstracts", "abu", "abuse", "ac", "academic", "academics", "academy", "acc", "accent", "accept", "acceptable", "acceptance",
			"accepted", "accepting", "accepts", "access", "accessed", "accessibility", "accessible", "accessing", "accessories",
			"accessory", "accident", "accidents", "accommodate", "accommodation", "accommodations", "accompanied", "accompanying",
			"accomplish", "accomplished", "accordance", "according", "accordingly", "account", "accountability", "accounting", "accounts",
			"about", "above", "abraham", "abroad", "abs", "absence", "absent", "absolute", "absolutely", "absorption", "abstract",
			"abstracts", "abu", "abuse", "ac", "academic", "academics", "academy", "acc", "accent", "accept", "acceptable", "acceptance",
			"abstracts", "abu", "abuse", "ac", "academic", "academics", "academy", "acc", "accent", "accept", "acceptable", "acceptance",
			"accepted", "accepting", "accepts", "access", "accessed", "accessibility", "accessible", "accessing", "accessories",
			"accessory", "accident", "accidents", "accommodate", "accommodation", "accommodations", "accompanied", "accompanying",
			"accepted", "accepting", "accepts", "access", "accessed", "accessibility", "accessible", "accessing", "accessories",
			"accreditation", "accredited", "accuracy", "accurate", "accurately", "accused", "acdbentity", "ace",
			"abstracts", "abu", "abuse", "ac", "academic", "academics", "academy", "acc", "accent", "accept", "acceptable", "acceptance",
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
				xl.Info("okay-" + t)

				if mathRand.Intn(100) > 75 { // log errors 25% of the time.
					l(context.Background()).Error("hey", errors.New("some-err-"+t))
					xl.Error("some-xl-error", errors.New("other-err-"+t))
				}
			}(tok)
		}
		wg.Wait()
	})
}
