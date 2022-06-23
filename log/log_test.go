package log

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	stdlibErrors "errors"
	stdLog "log"

	"github.com/komuw/goweb/errors"

	"github.com/akshayjshah/attest"
	"github.com/rs/zerolog"
	"github.com/sirupsen/logrus"
	"go.uber.org/zap"

	"go.uber.org/zap/zapcore"
)

func TestLogger(t *testing.T) {
	t.Parallel()

	t.Run("info level does not do anything", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(context.Background(), w, maxMsgs, true)
		l.Info(F{"one": "one"})

		attest.Zero(t, w.String())
	})

	t.Run("error logs immediately", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(context.Background(), w, maxMsgs, true)
		msg := "oops, Houston we got 99 problems."
		l.Error(errors.New(msg))

		attest.True(t, strings.Contains(w.String(), msg))
	})

	t.Run("info logs are flushed on error", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(context.Background(), w, maxMsgs, true)

		infoMsg := "hello world"
		l.Info(F{"what": infoMsg})
		errMsg := "oops, Houston we got 99 problems."
		l.Error(errors.New(errMsg))

		attest.True(t, strings.Contains(w.String(), infoMsg))
		attest.True(t, strings.Contains(w.String(), errMsg))
	})

	t.Run("neccesary fields added", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(context.Background(), w, maxMsgs, true)

		{
			infoMsg := "hello world"
			l.Info(F{"what": infoMsg})
			l.Error(errors.New("bad"))

			id := GetId(l.ctx)
			attest.True(t, strings.Contains(w.String(), id))
			attest.True(t, strings.Contains(w.String(), "level"))
			attest.True(t, strings.Contains(w.String(), "stack"))
			attest.True(t, strings.Contains(w.String(), "err"))
			attest.False(t, strings.Contains(w.String(), "line")) // line not added
		}

		{
			l = l.WithCaller()
			l.Info(F{"name": "john"})
			errMsg := "kimeumana"
			l.Error(errors.New(errMsg))

			id := GetId(l.ctx)
			attest.True(t, strings.Contains(w.String(), id))
			attest.True(t, strings.Contains(w.String(), "level"))
			attest.True(t, strings.Contains(w.String(), "stack"))
			attest.True(t, strings.Contains(w.String(), "err"))
			attest.True(t, strings.Contains(w.String(), "line")) // line added
		}
	})

	t.Run("logs are rotated", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(context.Background(), w, maxMsgs, true)

		for i := 0; i <= (maxMsgs + 4); i++ {
			infoMsg := "hello world" + " : " + fmt.Sprint(i)
			l.Info(F{"what": infoMsg})
		}
		errMsg := "oops, Houston we got 99 problems."
		l.Error(errors.New(errMsg))

		attest.False(t, strings.Contains(w.String(), "hello world : 1"))
		attest.False(t, strings.Contains(w.String(), "hello world : 2"))
		attest.False(t, strings.Contains(w.String(), "hello world : 5"))
		attest.True(t, strings.Contains(w.String(), "hello world : 6"))
		attest.True(t, strings.Contains(w.String(), "hello world : 7"))
		attest.True(t, strings.Contains(w.String(), errMsg))
	})

	t.Run("various ways of calling l.Error", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(context.Background(), w, maxMsgs, true)
		msg := "some-error"
		err := errors.New(msg)

		l.Error(err)
		l.Error(err, F{"one": "two"})
		l.Error(err, F{"three": "four"}, F{"five": "six"})
		l.Error(err, nil)
		l.Error(nil)
		l.Error(nil, F{"seven": "eight"})

		attest.True(t, strings.Contains(w.String(), msg))
		for _, v := range []string{"one", "two", "three", "four", "five", "six", "seven", "eight"} {
			attest.True(t, strings.Contains(w.String(), v), attest.Sprintf("`%s` not found", v))
		}
	})

	t.Run("WithCtx does not invalidate buffer", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(context.Background(), w, maxMsgs, true)
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
			attest.True(t, strings.Contains(w.String(), "hello world : 3"))
			attest.True(t, strings.Contains(w.String(), errMsg))
		}
	})

	t.Run("WithCaller does not invalidate buffer", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(context.Background(), w, maxMsgs, true)
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
			attest.True(t, strings.Contains(w.String(), "hello world : 3"))
			attest.True(t, strings.Contains(w.String(), errMsg))
		}
	})

	t.Run("WithFields", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(context.Background(), w, maxMsgs, true)
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
			attest.True(t, strings.Contains(w.String(), v))
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
			attest.True(t, strings.Contains(w.String(), v))
		}
	})

	t.Run("WithImmediate logs immediately", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		msg := "hello world"
		l := New(context.Background(), w, 2, true).WithImmediate()
		l.Info(F{"msg": msg})

		attest.True(t, strings.Contains(w.String(), msg))
	})

	t.Run("interop with stdlibLog", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		msg := "hello world"
		l := New(context.Background(), w, 2, true)
		stdLogger := stdLog.New(l, "stdlib", stdLog.Lshortfile)
		stdLogger.Println(msg)

		attest.True(t, strings.Contains(w.String(), msg))
	})

	t.Run("get stdlibLog", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		msg := "hey what up?"
		l := New(context.Background(), w, 2, true)
		stdLogger := l.StdLogger()
		stdLogger.Println(msg)
		fmt.Println(w.String())
		attest.True(t, strings.Contains(w.String(), msg))
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(context.Background(), w, maxMsgs, true)

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
				l.Info(F{"four": "four" + t})
				wg.Done()
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
pkg: github.com/komuw/goweb/log
cpu: Intel(R) Core(TM) i7-10510U CPU @ 1.80GHz

BenchmarkBestCase/goweb/log-8            889.8 ns/op	      41 B/op	       2 allocs/op
BenchmarkBestCase/rs/zerolog-8           11_360 ns/op	     153 B/op	       0 allocs/op
BenchmarkBestCase/Zap-8                  22_620 ns/op	     343 B/op	       1 allocs/op
BenchmarkBestCase/sirupsen/logrus-8      32_635 ns/op	    2112 B/op	      26 allocs/op
*/
// The above benchmark is unfair to the others since goweb/log is not logging to a io.writer when all its logging are Info logs.

/*
BenchmarkAverageCase/rs/zerolog-8        12_513 ns/op	     153 B/op	       0 allocs/op
BenchmarkAverageCase/Zap-8               21_818 ns/op	     348 B/op	       1 allocs/op
BenchmarkAverageCase/sirupsen/logrus-8   33_401 ns/op	    1961 B/op	      26 allocs/op
BenchmarkAverageCase/goweb/log-8         172_514 ns/op	    3571 B/op	      42 allocs/op
*/

/*
BenchmarkWorstCase/rs/zerolog-8          17_867 ns/op	     303 B/op	       0 allocs/op
BenchmarkWorstCase/Zap-8                 26_665 ns/op	     688 B/op	       2 allocs/op
BenchmarkWorstCase/sirupsen/logrus-8     57_033 ns/op	    3663 B/op	      53 allocs/op
BenchmarkWorstCase/goweb/log-8           333_297 ns/op	   10025 B/op	     103 allocs/op
*/
// The above benchmark is 'more representative' since this time round, goweb/log is writing to io.writer for every invocation.

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

func newGoWebLogger() Logger {
	maxMsgs := 50_000
	return New(
		context.Background(),
		io.Discard,
		maxMsgs,
		true,
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

func BenchmarkBestCase(b *testing.B) {
	f, sl := getMessage()
	str := fmt.Sprintf("%s", sl)
	b.Logf("best case") // best-case because goweb/log does not log if it is not error level

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

	b.Run("goweb/log", func(b *testing.B) {
		l := newGoWebLogger()
		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			l.Info(f)
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

	b.Run("goweb/log", func(b *testing.B) {
		l := newGoWebLogger()
		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			l.Info(f)
			if rand.Intn(100) >= 99 {
				l.Error(logErr)
			}
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

	b.Run("goweb/log", func(b *testing.B) {
		l := newGoWebLogger()
		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			l.Info(f)
			l.Error(logErr, f)
		}
	})
}

//////////////////////////////////////////////////////////////////////// BENCHMARKS ////////////////////////////////////////////////////////////////////////
