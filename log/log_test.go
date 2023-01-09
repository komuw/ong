package log

import (
	"bytes"
	"context"
	stdlibErrors "errors"
	"fmt"
	"io"
	stdLog "log"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/akshayjshah/attest"
	"github.com/komuw/ong/errors"
	"github.com/rs/zerolog"
	"github.com/sirupsen/logrus"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestCircleBuf(t *testing.T) {
	t.Parallel()

	t.Run("it stores", func(t *testing.T) {
		t.Parallel()

		maxSize := 4
		c := newCirleBuf(maxSize)
		c.store(F{"msg": "one"})
		c.store(F{"msg": "two"})

		val1, ok := c.buf[0]["msg"].(string)
		attest.True(t, ok)
		attest.Equal(t, val1, "one")

		val2, ok := c.buf[1]["msg"].(string)
		attest.True(t, ok)
		attest.Equal(t, val2, "two")

		attest.Equal(t, len(c.buf), 2)
		attest.Equal(t, cap(c.buf), 4)
	})

	t.Run("does not exceed maxsize", func(t *testing.T) {
		t.Parallel()

		maxSize := 8
		c := newCirleBuf(maxSize)
		for i := 0; i <= (13 * maxSize); i++ {
			x := fmt.Sprint(i)
			c.store(F{x: x})
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
			c.store(F{"msg": x})
			attest.True(t, len(c.buf) <= maxSize)
			attest.True(t, cap(c.buf) <= maxSize)
		}
		attest.True(t, len(c.buf) <= maxSize)
		attest.True(t, cap(c.buf) <= maxSize)

		val1, ok := c.buf[1]["msg"].(string)
		attest.True(t, ok)
		attest.Equal(t, val1, "29")
		val2, ok := c.buf[2]["msg"].(string)
		attest.True(t, ok)
		attest.Equal(t, val2, "30")
	})

	t.Run("reset", func(t *testing.T) {
		t.Parallel()

		maxSize := 80
		c := newCirleBuf(maxSize)
		for i := 0; i <= (13 * maxSize); i++ {
			x := fmt.Sprint(i)
			c.store(F{x: x})
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
		l.Info(F{"one": "one"})

		attest.Zero(t, w.String())
	})

	t.Run("error logs immediately", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(w, maxMsgs)
		msg := "oops, Houston we got 99 problems."
		l.Error(errors.New(msg))

		attest.Subsequence(t, w.String(), msg)
	})

	t.Run("info logs are flushed on error", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(w, maxMsgs)

		infoMsg := "hello world"
		l.Info(F{"what": infoMsg, "ok": "ak&dHyS>47K"})
		errMsg := "oops, Houston we got 99 problems."
		l.Error(errors.New(errMsg))

		attest.Subsequence(t, w.String(), infoMsg)
		attest.Subsequence(t, w.String(), errMsg)
		// special characters are not quoted.
		attest.Subsequence(t, w.String(), "&")
		attest.Subsequence(t, w.String(), ">")
	})

	t.Run("neccesary fields added", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(w, maxMsgs)

		{
			infoMsg := "hello world"
			l.Info(F{"what": infoMsg})
			l.Error(errors.New("bad"))

			id := l.logId
			attest.NotZero(t, id)
			attest.Subsequence(t, w.String(), id)
			attest.Subsequence(t, w.String(), "level")
			attest.Subsequence(t, w.String(), "stack")
			attest.Subsequence(t, w.String(), "err")
			attest.False(t, strings.Contains(w.String(), "line")) // line not added
		}

		{
			l = l.WithCaller()
			l.Info(F{"name": "john"})
			errMsg := "kimeumana"
			l.Error(errors.New(errMsg))

			id := l.logId
			attest.NotZero(t, id)
			attest.Subsequence(t, w.String(), id)
			attest.Subsequence(t, w.String(), "level")
			attest.Subsequence(t, w.String(), "stack")
			attest.Subsequence(t, w.String(), "err")
			attest.Subsequence(t, w.String(), "line") // line added
		}
	})

	t.Run("logs are rotated", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(w, maxMsgs)

		for i := 0; i <= (maxMsgs + 4); i++ {
			infoMsg := "hello world" + " : " + fmt.Sprint(i)
			l.Info(F{"what": infoMsg})
		}
		errMsg := "oops, Houston we got 99 problems."
		l.Error(errors.New(errMsg))

		attest.False(t, strings.Contains(w.String(), "hello world : 1"))
		attest.False(t, strings.Contains(w.String(), "hello world : 2"))
		attest.False(t, strings.Contains(w.String(), "hello world : 5"))
		attest.Subsequence(t, w.String(), "hello world : 6")
		attest.Subsequence(t, w.String(), "hello world : 7")
		attest.Subsequence(t, w.String(), errMsg)
	})

	t.Run("various ways of calling l.Error", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(w, maxMsgs)
		msg := "some-error"
		err := errors.New(msg)

		l.Error(err)
		l.Error(err, F{"one": "two"})
		l.Error(err, F{"three": "four"}, F{"five": "six"})
		l.Error(err, nil)
		l.Error(nil)
		l.Error(nil, F{"seven": "eight"})

		attest.Subsequence(t, w.String(), msg)
		for _, v := range []string{"one", "two", "three", "four", "five", "six", "seven", "eight"} {
			attest.Subsequence(t, w.String(), v, attest.Sprintf("`%s` not found", v))
		}
	})

	t.Run("WithCtx does not invalidate buffer", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(w, maxMsgs)
		{
			for i := 0; i <= (maxMsgs); i++ {
				infoMsg := "hello world" + " : " + fmt.Sprint(i)
				l.Info(F{"what": infoMsg})
			}
			attest.False(t, strings.Contains(w.String(), "hello world : 0"))
			attest.False(t, strings.Contains(w.String(), "hello world : 1"))
			attest.False(t, strings.Contains(w.String(), "hello world : 2"))
			attest.False(t, strings.Contains(w.String(), "hello world : 3"))
		}

		{
			l = l.WithCtx(context.Background())
			errMsg := "oops, Houston we got 99 problems."
			l.Error(errors.New(errMsg))

			attest.False(t, strings.Contains(w.String(), "hello world : 0"))
			attest.False(t, strings.Contains(w.String(), "hello world : 1"))
			attest.False(t, strings.Contains(w.String(), "hello world : 2"))
			attest.Subsequence(t, w.String(), "hello world : 3")
			attest.Subsequence(t, w.String(), errMsg)
		}
	})

	t.Run("WithCaller does not invalidate buffer", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(w, maxMsgs)
		{
			for i := 0; i <= (maxMsgs); i++ {
				infoMsg := "hello world" + " : " + fmt.Sprint(i)
				l.Info(F{"what": infoMsg})
			}
			attest.False(t, strings.Contains(w.String(), "hello world : 0"))
			attest.False(t, strings.Contains(w.String(), "hello world : 1"))
			attest.False(t, strings.Contains(w.String(), "hello world : 2"))
			attest.False(t, strings.Contains(w.String(), "hello world : 3"))
		}

		{
			l = l.WithCaller()
			errMsg := "oops, Houston we got 99 problems."
			l.Error(errors.New(errMsg))

			attest.False(t, strings.Contains(w.String(), "hello world : 0"))
			attest.False(t, strings.Contains(w.String(), "hello world : 1"))
			attest.False(t, strings.Contains(w.String(), "hello world : 2"))
			attest.Subsequence(t, w.String(), "hello world : 3")
			attest.Subsequence(t, w.String(), errMsg)
		}
	})

	t.Run("WithFields", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(w, maxMsgs)
		flds := F{"version": "v0.1.2", "env": "prod", "service": "web-commerce"}
		l = l.WithFields(flds)

		msg := "hello"
		l.Info(F{"msg": msg})
		errMsg := "oops, Houston we got 99 problems."
		l.Error(errors.New(errMsg))

		for _, v := range []string{
			"version",
			"v0.1.2",
			"web-commerce",
			msg,
			errMsg,
		} {
			attest.Subsequence(t, w.String(), v)
		}
		attest.Equal(t, l.flds, flds)

		newFlds := F{"okay": "yes", "country": "Norway"}
		l = l.WithFields(newFlds)
		newErrMsg := "new error"
		l.Error(errors.New(newErrMsg))
		// asserts that the `l.flds` maps does not grow without bound.
		attest.Equal(t, l.flds, newFlds)
		for _, v := range []string{
			"okay",
			"yes",
			"Norway",
			msg,
			newErrMsg,
		} {
			attest.Subsequence(t, w.String(), v)
		}
	})

	t.Run("WithImmediate logs immediately", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		msg := "hello world"
		l := New(w, 2).WithImmediate()
		l.Info(F{"msg": msg})

		attest.Subsequence(t, w.String(), msg)
	})

	t.Run("interop with stdlibLog", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		msg := "hello world"
		l := New(w, 2)
		stdLogger := stdLog.New(l, "stdlib", stdLog.Lshortfile)
		stdLogger.Println(msg)

		attest.Subsequence(t, w.String(), msg)
	})

	t.Run("get stdlibLog", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		msg := "hey what up?"
		l := New(w, 2)
		stdLogger := l.StdLogger()
		stdLogger.Println(msg)
		attest.Subsequence(t, w.String(), msg)
	})

	t.Run("WithCaller uses correct line", func(t *testing.T) {
		t.Parallel()

		{
			w := &bytes.Buffer{}
			msg := "hey what up?"
			l := New(w, 2)
			l.WithCaller().WithImmediate().Info(F{"msg": msg})
			attest.Subsequence(t, w.String(), msg)
			attest.Subsequence(t, w.String(), "ong/log/log_test.go:374")
		}

		{
			// for stdlib we disable caller info, since it would otherwise
			// point to `ong/log/log.go` as the caller.
			w := &bytes.Buffer{}
			msg := "hey what up?"
			l := New(w, 2)
			l.WithCaller().StdLogger().Println(msg)
			attest.Subsequence(t, w.String(), msg)
			attest.False(t, strings.Contains(w.String(), "ong/log/log_test.go"))
		}

		{
			w := &bytes.Buffer{}
			msg := "hey what up?"
			l := New(w, 2).WithCaller()
			stdLogger := stdLog.New(l, "stdlib", 0)
			stdLogger.Println(msg)
			attest.Subsequence(t, w.String(), msg)
			attest.False(t, strings.Contains(w.String(), "ong/log/log_test.go"))
		}
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(w, maxMsgs)

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
				l.Info(F{"one": "one" + t})
			}(tok)
		}

		for _, tok := range tokens {
			go func(t string) {
				l.Error(errors.New("bad" + t))
			}(tok)
		}

		for _, tok := range tokens {
			go func(t string) {
				l.Error(errors.New("bad-two" + t))
			}(tok)
		}

		wg := &sync.WaitGroup{}
		for _, tok := range tokens {
			wg.Add(1)
			go func(t string) {
				defer wg.Done()
				l.Info(F{"four": "four" + t})
			}(tok)
		}
		wg.Wait()
	})
}

//////////////////////////////////////////////////////////////////////// BENCHMARKS ////////////////////////////////////////////////////////////////////////
// The benchmarks code here is insipired by(or taken from):
//   (a) https://github.com/uber-go/zap/tree/v1.21.0/benchmarks whose license(MIT) can be found here: https://github.com/uber-go/zap/blob/v1.21.0/LICENSE.txt

// note: Im not making any claims about which is faster or not.
/*
goos: linux
goarch: amd64
pkg: github.com/komuw/ong/log
cpu: Intel(R) Core(TM) i7-10510U CPU @ 1.80GHz

BenchmarkBestCase/no_logger-8             6.950 ns/op	       0 B/op	       0 allocs/op
BenchmarkBestCase/ong/log-8               902.4 ns/op	      56 B/op	       3 allocs/op
BenchmarkBestCase/rs/zerolog-8            11_260 ns/op	     150 B/op	       0 allocs/op
BenchmarkBestCase/Zap-8  	              22_341 ns/op	     343 B/op	       1 allocs/op
BenchmarkBestCase/sirupsen/logrus-8       32_260 ns/op	    2042 B/op	      26 allocs/op
*/
// The above benchmark is unfair to the others since ong/log is not logging to a io.writer when all its logging are Info logs.

/*
BenchmarkAverageCase/no_logger-8           6.797 ns/op	       0 B/op	       0 allocs/op
BenchmarkAverageCase/rs/zerolog-8          12_249 ns/op	     152 B/op	       0 allocs/op
BenchmarkAverageCase/Zap-8                 21_539 ns/op	     348 B/op	       1 allocs/op
BenchmarkAverageCase/sirupsen/logrus-8     33_543 ns/op	    1962 B/op	      26 allocs/op
BenchmarkAverageCase/ong/log-8             75_640 ns/op	    3369 B/op	      42 allocs/op
*/

/*
BenchmarkWorstCase/no_logger-8             6.806 ns/op	       0 B/op	       0 allocs/op
BenchmarkWorstCase/rs/zerolog-8            17_721 ns/op	     305 B/op	       1 allocs/op
BenchmarkWorstCase/Zap-8                   26_612 ns/op	     690 B/op	       2 allocs/op
BenchmarkWorstCase/sirupsen/logrus-8       56_562 ns/op	    3664 B/op	      53 allocs/op
BenchmarkWorstCase/ong/log-8               167_518 ns/op	8362 B/op	      95 allocs/op
*/
// The above benchmark is 'more representative' since this time round, ong/log is writing to io.writer for every invocation.

func newZerolog() zerolog.Logger {
	return zerolog.New(io.Discard).With().Timestamp().Logger()
}

func newLogrus() *logrus.Logger {
	return &logrus.Logger{
		Out:       io.Discard,
		Formatter: new(logrus.JSONFormatter),
		Hooks:     make(logrus.LevelHooks),
		Level:     logrus.DebugLevel,
	}
}

// implements zap's `ztest.Discarder{}` which is internal.
type Discarder struct {
	io.Writer
}

func (d Discarder) Sync() error { return nil }

func newZapLogger(lvl zapcore.Level) *zap.Logger {
	ec := zap.NewProductionEncoderConfig()
	ec.EncodeDuration = zapcore.NanosDurationEncoder
	ec.EncodeTime = zapcore.EpochNanosTimeEncoder
	enc := zapcore.NewJSONEncoder(ec)
	return zap.New(zapcore.NewCore(
		enc,
		Discarder{io.Discard},
		lvl,
	))
}

func newOngLogger() Logger {
	maxMsgs := 50_000
	return New(
		io.Discard,
		maxMsgs,
	)
}

func getMessage() (F, []string) {
	type car struct {
		mft  string
		date uint64
	}
	c := car{mft: "Toyota", date: uint64(1994)}
	f := F{
		"some-random-id": "kad8184dHjekI1ESL",
		"age":            34,
		"name":           "John Snow",
		"gender":         "male",
		"company":        "ACME INC",
		"email":          "sandersgonzalez@pivitol.com",
		"phone":          "+1 (914) 563-2007",
		"startdate":      time.Now(),
		"height":         float64(89.22),
		"car_length":     float32(123.8999),
		"carVal":         c,
		"carPtr":         &c,
	}

	sl := make([]string, 0, len(f))

	for k, v := range f {
		sl = append(sl, k)
		sl = append(sl, fmt.Sprintf("%v", v))
	}

	return f, sl
}

func noOpFunc(f F) {
	// func used in the `no logger` benchmark.
	_ = f
}

func BenchmarkBestCase(b *testing.B) {
	f, sl := getMessage()
	str := fmt.Sprintf("%s", sl)
	b.Logf("best case") // best-case because ong/log does not log if it is not error level

	b.Run("Zap", func(b *testing.B) {
		l := newZapLogger(zap.DebugLevel)
		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			l.Info(str)
		}
	})

	b.Run("sirupsen/logrus", func(b *testing.B) {
		l := newLogrus()
		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			l.Info(str)
		}
	})

	b.Run("rs/zerolog", func(b *testing.B) {
		l := newZerolog()
		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			l.Info().Msg(str)
		}
	})

	b.Run("ong/log", func(b *testing.B) {
		l := newOngLogger()
		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			l.Info(f)
		}
	})

	b.Run("no logger", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			noOpFunc(f)
		}
	})
}

func BenchmarkAverageCase(b *testing.B) {
	f, sl := getMessage()
	str := fmt.Sprintf("%s", sl)
	logErr := stdlibErrors.New("hey")

	rand.Seed(time.Now().UnixNano())

	b.Logf("average case")

	b.Run("Zap", func(b *testing.B) {
		l := newZapLogger(zap.DebugLevel)
		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			l.Info(str)
			if rand.Intn(100) >= 99 {
				l.Error(logErr.Error())
			}
		}
	})

	b.Run("sirupsen/logrus", func(b *testing.B) {
		l := newLogrus()
		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			l.Info(str)
			if rand.Intn(100) >= 99 {
				l.Error(logErr.Error())
			}
		}
	})

	b.Run("rs/zerolog", func(b *testing.B) {
		l := newZerolog()
		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			l.Info().Msg(str)
			if rand.Intn(100) >= 99 {
				l.Error().Msg(logErr.Error())
			}
		}
	})

	b.Run("ong/log", func(b *testing.B) {
		l := newOngLogger()
		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			l.Info(f)
			if rand.Intn(100) >= 99 {
				l.Error(logErr)
			}
		}
	})

	b.Run("no logger", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			noOpFunc(f)
		}
	})
}

func BenchmarkWorstCase(b *testing.B) {
	f, sl := getMessage()
	str := fmt.Sprintf("%s", sl)
	logErr := stdlibErrors.New("hey")

	b.Logf("worst case")

	b.Run("Zap", func(b *testing.B) {
		l := newZapLogger(zap.DebugLevel)
		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			l.Info(str)
			l.Error(logErr.Error())
		}
	})

	b.Run("sirupsen/logrus", func(b *testing.B) {
		l := newLogrus()
		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			l.Info(str)
			l.Error(logErr.Error())
		}
	})

	b.Run("rs/zerolog", func(b *testing.B) {
		l := newZerolog()
		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			l.Info().Msg(str)
			l.Error().Msg(logErr.Error())
		}
	})

	b.Run("ong/log", func(b *testing.B) {
		l := newOngLogger()
		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			l.Info(f)
			l.Error(logErr, f)
		}
	})

	b.Run("no logger", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			noOpFunc(f)
		}
	})
}

//////////////////////////////////////////////////////////////////////// BENCHMARKS ////////////////////////////////////////////////////////////////////////
