package log

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	mathRand "math/rand"
	"strings"
	"sync"
	"testing"

	ongErrors "github.com/komuw/ong/errors"
	"github.com/komuw/ong/internal/octx"

	"go.akshayshah.org/attest"
	"go.uber.org/goleak"
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
		l := New(context.Background(), w, maxMsgs)
		l.Info("hey", "one", "one")

		attest.Zero(t, w.String())
	})

	t.Run("error logs immediately", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(context.Background(), w, maxMsgs)
		msg := "oops, Houston we got 99 problems."
		l.Error(msg, errors.New(msg))

		attest.Subsequence(t, w.String(), msg)

		{
			// it can log nils.
			l.Info(msg, "someValue", nil)
			l.Error(msg, "nil", nil)
		}
	})

	t.Run("info logs are flushed on error", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(context.Background(), w, maxMsgs)

		logger := l
		infoMsg := "hello world"
		logger.Info(infoMsg, "what", "ok", "passwd", "ak&dHyS>47K")
		errMsg := "oops, Houston we got 99 problems."
		logger.Error("some-error", "err", errors.New(errMsg))

		_, ok := logger.Handler().(*handler)
		attest.True(t, ok)

		attest.Subsequence(t, w.String(), infoMsg)
		attest.Subsequence(t, w.String(), errMsg)

		// special characters are not quoted.
		attest.Subsequence(t, w.String(), "&")
		attest.Subsequence(t, w.String(), ">")
	})

	t.Run("neccesary fields added", func(t *testing.T) {
		t.Parallel()

		{
			w := &bytes.Buffer{}
			maxMsgs := 3
			l := New(context.Background(), w, maxMsgs)

			infoMsg := "hello world"
			l.Info(infoMsg)
			l.Error("some-err", errors.New("this-ting-is-bad"))

			attest.Subsequence(t, w.String(), logIDFieldName)
			attest.Subsequence(t, w.String(), "level")
			attest.Subsequence(t, w.String(), "source")
			attest.Subsequence(t, w.String(), "this-ting-is-bad")
		}

		{
			w := &bytes.Buffer{}
			maxMsgs := 3
			l := New(context.Background(), w, maxMsgs)

			infoMsg := "hello world"
			l.Info(infoMsg)
			l.Error("some-ong-err", "err", ongErrors.New("bad"))

			attest.Subsequence(t, w.String(), logIDFieldName)
			attest.Subsequence(t, w.String(), "stack")
			attest.Subsequence(t, w.String(), "log_test.go:182") // stacktrace added.
		}
	})

	t.Run("logs are rotated", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(context.Background(), w, maxMsgs)

		for i := 0; i <= (maxMsgs + 4); i++ {
			infoMsg := "hello world" + " : " + fmt.Sprint(i)
			l.Info(infoMsg)
		}
		errMsg := "oops, Houston we got 99 problems."
		l.Error("somer-error", errors.New(errMsg))

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

		l1 := New(context.Background(), w, maxMsgs)
		_, ok := l1.Handler().(*handler)
		attest.True(t, ok)
		l1.Error("hey1", errors.New("cool1"))

		attest.Subsequence(t, w.String(), "hey1")
		attest.Subsequence(t, w.String(), logIDFieldName)
		h, ok := l1.Handler().(*handler)
		attest.True(t, ok)
		attest.Equal(t, len(h.cBuf.buf), 0)

		w.Reset() // clear buffer.

		l2 := l1.With("category", "load_balancers")
		l2.Error("hey2", errors.New("cool2"))
		attest.Subsequence(t, w.String(), "hey2")
		attest.False(t, strings.Contains(w.String(), "hey1")) // hey1 is not loggged here.
		attest.Subsequence(t, w.String(), logIDFieldName)
	})

	t.Run(
		// See: https://github.com/komuw/ong/issues/316
		"logID not duplicated",
		func(t *testing.T) {
			t.Parallel()

			w := &bytes.Buffer{}
			maxMsgs := 3

			msg := "hey"
			l := New(context.Background(), w, maxMsgs)
			l.Error(msg)

			attest.Equal(t, strings.Count(w.String(), logIDFieldName), 1)
			attest.Equal(t, strings.Count(w.String(), slog.MessageKey), 1)
			attest.Equal(t, strings.Count(w.String(), msg), 1)
			attest.Equal(t, strings.Count(w.String(), "time"), 1)
			attest.Equal(t, strings.Count(w.String(), "level"), 1)
		},
	)

	t.Run("New context does not invalidate buffer", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(context.Background(), w, maxMsgs)
		{
			logger1 := l
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
			logger2 := l.With("name", "Kim")
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
		msg1 := "messageOne"
		l := New(context.Background(), w, 3)

		l.InfoContext(ctx, msg1)
		l.ErrorContext(ctx, "hey1", "err1", errors.New("badTingOne"))
		attest.Subsequence(t, w.String(), msg1)
		attest.Equal(t,
			strings.Count(w.String(), logIDFieldName),
			// One for the messageOne info log the other for the badTingOne error log
			2,
		)

		newId := "NEW-id-adh4e92427dajd"
		ctx = context.WithValue(ctx, octx.LogCtxKey, newId)
		l.ErrorContext(ctx, "hey2", "err2", errors.New("badTingTwo"))
		attest.Subsequence(t, w.String(), newId)
		attest.Equal(t,
			strings.Count(w.String(), logIDFieldName),
			// One each for messageOne, badTingOne & badTingTwo.
			3,
		)

		newId3 := "NEW-id3-alas"
		ctx = context.WithValue(ctx, octx.LogCtxKey, newId3)
		l.ErrorContext(ctx, "hey3", "err3", errors.New("badTingThree"))
		attest.Subsequence(t, w.String(), newId)
		attest.Equal(t,
			strings.Count(w.String(), logIDFieldName),
			// One each for messageOne, badTingOne, badTingTwo & badTingThree.
			4,
		)
	})

	t.Run("logID reused with no context", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		msg1 := "messageOne"
		msg2 := "messageTwo"
		l := New(context.Background(), w, 30)

		l.Info(msg1)
		l.Info(msg2)
		l.Error("badTingOne", "err1", errors.New("badTingOne"))

		fmt.Println("\t cool: ")
		fmt.Println(w.String())
		attest.Subsequence(t, w.String(), msg1)
		attest.Subsequence(t, w.String(), msg2)
		attest.Subsequence(t, w.String(), "badTingOne")

		parts := strings.Split(w.String(), "\n")
		attest.Equal(t, len(parts), 4) // one of them is an empty line.

		type logStruct struct {
			Time  string `json:"time"`
			Msg   string `json:"msg"`
			LogID string `json:"logID"`
		}

		resp1 := &logStruct{}
		attest.Ok(t, json.Unmarshal([]byte(parts[0]), resp1))

		resp2 := &logStruct{}
		attest.Ok(t, json.Unmarshal([]byte(parts[1]), resp2))

		resp3 := &logStruct{}
		attest.Ok(t, json.Unmarshal([]byte(parts[2]), resp3))

		// All the logIDs should be the same.
		attest.NotZero(t, resp1.LogID)
		attest.Equal(t, resp1.LogID, resp2.LogID)
		attest.Equal(t, resp1.LogID, resp3.LogID)
	})

	t.Run("stdlibLog", func(t *testing.T) {
		t.Parallel()

		{ // normal loglevels go through circular buffer.
			w := &bytes.Buffer{}
			msg := "hey what up?"
			l := New(context.Background(), w, 2)

			stdLogger := slog.NewLogLogger(l.Handler(), slog.LevelInfo)
			stdLogger.Println(msg)
			attest.Zero(t, w.String())
		}

		{ // `LevelImmediate` loglevel is logged ASAP.
			w := &bytes.Buffer{}
			msg := "hey what up?"
			l := New(context.Background(), w, 2)

			stdLogger := slog.NewLogLogger(l.Handler(), LevelImmediate)
			stdLogger.Println(msg)
			attest.Subsequence(t, w.String(), msg)
			attest.Subsequence(t, w.String(), `log_test.go`)
			attest.Subsequence(t, w.String(), `log_test.go:390`)
			attest.True(t, LevelImmediate < 0) // otherwise it will trigger `log.handler` to flush all logs, which we dont want.
		}
	})

	t.Run("WithID", func(t *testing.T) {
		t.Parallel()

		type logStruct struct {
			Time  string `json:"time"`
			Msg   string `json:"msg"`
			LogID string `json:"logID"`
		}

		{
			w := &bytes.Buffer{}
			l := New(context.Background(), w, 200)

			msg1 := "hello"
			l.Info(msg1)

			l = WithID(context.Background(), l)
			l.Error("badTingOne", "err1", errors.New("badTingOne"))
			l.Error("badTingTwo", "err2", errors.New("badTingTwo"))

			parts := strings.Split(w.String(), "\n")
			attest.Equal(t, len(parts), 4) // one of them is an empty line.

			resp1 := &logStruct{}
			attest.Ok(t, json.Unmarshal([]byte(parts[0]), resp1))

			resp2 := &logStruct{}
			attest.Ok(t, json.Unmarshal([]byte(parts[1]), resp2))

			resp3 := &logStruct{}
			attest.Ok(t, json.Unmarshal([]byte(parts[2]), resp3))

			// All the logIDs should be the same.
			attest.NotZero(t, resp1.LogID)
			attest.Equal(t, resp1.LogID, resp2.LogID)
			attest.Equal(t, resp1.LogID, resp3.LogID)
		}

		{
			w := &bytes.Buffer{}
			l := New(context.Background(), w, 200)

			msg1 := "hello"
			l.Info(msg1)

			newId := "NEW-id-adh4e92427dajd"
			ctx := context.WithValue(context.Background(), octx.LogCtxKey, newId)
			l = WithID(ctx, l)
			l.Error("badTingOne", "err1", errors.New("badTingOne"))
			l.Error("badTingTwo", "err2", errors.New("badTingTwo"))

			parts := strings.Split(w.String(), "\n")
			attest.Equal(t, len(parts), 4) // one of them is an empty line.

			resp1 := &logStruct{}
			attest.Ok(t, json.Unmarshal([]byte(parts[0]), resp1))

			resp2 := &logStruct{}
			attest.Ok(t, json.Unmarshal([]byte(parts[1]), resp2))

			resp3 := &logStruct{}
			attest.Ok(t, json.Unmarshal([]byte(parts[2]), resp3))

			// The first log shouldn't have same id with the other two.
			attest.NotZero(t, resp1.LogID)
			attest.NotZero(t, resp2.LogID)
			attest.NotEqual(t, resp1.LogID, resp2.LogID)
			attest.Equal(t, resp2.LogID, resp3.LogID)
		}
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 12
		l := New(context.Background(), w, maxMsgs)

		xl := l

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
				l.Info("one" + t)
				xl.Info("one" + t)
			}(tok)
		}

		for _, tok := range tokens {
			go func(t string) {
				l.Error("some-error", errors.New("bad"+t))
			}(tok)
		}

		for _, tok := range tokens {
			go func(t string) {
				l.Error("some-other-error", errors.New("bad-two"+t))
			}(tok)
		}

		wg := &sync.WaitGroup{}
		for _, tok := range tokens {
			wg.Add(1)
			go func(t string) {
				defer wg.Done()
				l.Info("four" + t)
				xl.Info("okay-" + t)

				if mathRand.Intn(100) > 75 { // log errors 25% of the time.
					l.Error("hey", errors.New("some-err-"+t))
					xl.Error("some-xl-error", errors.New("other-err-"+t))
				}
			}(tok)
		}
		wg.Wait()
	})
}

func TestGrouping(t *testing.T) {
	t.Parallel()

	{
		ctx := context.Background()
		w := &bytes.Buffer{}
		msg1 := "messageOne"
		l := New(context.Background(), w, 300)

		l = l.With("app", "mpesa")

		l.InfoContext(ctx, msg1)
		l.ErrorContext(ctx, "hey1", "err1", errors.New("badTingOne"))
		attest.Subsequence(t, w.String(), logIDFieldName)
		attest.Subsequence(t, w.String(), msg1)
		attest.Subsequence(t, w.String(), "mpesa")
		attest.Equal(t,
			strings.Count(w.String(), logIDFieldName),
			// One for the messageOne info log the other for the badTingOne error log
			2,
		)

		newId := "NEW-id-adh4e92427dajd"
		ctx = context.WithValue(ctx, octx.LogCtxKey, newId)
		l.ErrorContext(ctx, "hey2", "err2", errors.New("badTingTwo"))
		attest.Subsequence(t, w.String(), logIDFieldName)
		attest.Subsequence(t, w.String(), newId)
		attest.Subsequence(t, w.String(), "mpesa")
		attest.Equal(t,
			strings.Count(w.String(), logIDFieldName),
			// One each for messageOne, badTingOne & badTingTwo.
			3,
		)

		l.ErrorContext(ctx, "hey3", "err3", errors.New("badTingThree"))
		attest.Subsequence(t, w.String(), logIDFieldName)
		attest.Subsequence(t, w.String(), newId)
		attest.Subsequence(t, w.String(), "mpesa")
		attest.Equal(t,
			strings.Count(w.String(), logIDFieldName),
			// One each for messageOne, badTingOne, badTingTwo & badTingThree.
			4,
		)
	}

	{
		ctx := context.Background()
		w := &bytes.Buffer{}
		msg1 := "messageOne"
		l := New(context.Background(), w, 300)

		l = l.With("app", "mpesa")
		l = l.WithGroup("af-south-1")

		l.InfoContext(ctx, msg1)
		l.ErrorContext(ctx, "hey1", "err1", errors.New("badTingOne"))
		attest.Subsequence(t, w.String(), logIDFieldName)
		attest.Subsequence(t, w.String(), msg1)
		attest.Subsequence(t, w.String(), "mpesa")
		attest.Subsequence(t, w.String(), "af-south-1")
		attest.Equal(t,
			strings.Count(w.String(), logIDFieldName),
			// One for the messageOne info log the other for the badTingOne error log
			2,
		)

		newId := "NEW-id-adh4e92427dajd"
		ctx = context.WithValue(ctx, octx.LogCtxKey, newId)
		l.ErrorContext(ctx, "hey2", "err2", errors.New("badTingTwo"))
		attest.Subsequence(t, w.String(), logIDFieldName)
		attest.Subsequence(t, w.String(), newId)
		attest.Subsequence(t, w.String(), "mpesa")
		attest.Subsequence(t, w.String(), "af-south-1")
		attest.Equal(t,
			strings.Count(w.String(), logIDFieldName),
			// One each for messageOne, badTingOne & badTingTwo.
			3,
		)

		l.ErrorContext(ctx, "hey3", "err3", errors.New("badTingThree"))
		attest.Subsequence(t, w.String(), logIDFieldName)
		attest.Subsequence(t, w.String(), newId)
		attest.Subsequence(t, w.String(), "mpesa")
		attest.Subsequence(t, w.String(), "af-south-1")
		attest.Equal(t,
			strings.Count(w.String(), logIDFieldName),
			// One each for messageOne, badTingOne, badTingTwo & badTingThree.

			4,
		)
	}

	{ // groups for an empty Record.
		w := &bytes.Buffer{}
		l := New(context.Background(), w, 3)
		l.With("a", "b").WithGroup("G").With("c", "d").WithGroup("H").Error("hello-cool")
		attest.Subsequence(t, w.String(), logIDFieldName)
		attest.Equal(t,
			strings.Count(w.String(), logIDFieldName),
			1,
		)
	}
}
