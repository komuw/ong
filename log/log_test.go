package log

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/akshayjshah/attest"
	"github.com/komuw/goweb/errors"
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

			id := getLogId(l.ctx)
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

			id := getLogId(l.ctx)
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

func newGoWebLogger() logger {
	maxMsgs := 100_000
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

func BenchmarkTODO(b *testing.B) {

	f, sl := getMessage()
	_ = f
	str := fmt.Sprintf("%s", sl)

	// gowebLog := New(
	// 	context.Background(),
	// 	os.Stdout,
	// 	100,
	// 	true,
	// )
	// gowebLog.Info(f)
	// gowebLog.Error(errors.New("blah"))

	// zLog := zerolog.New(os.Stdout).With().Timestamp().Logger()
	// zLog.Info().Msgf("%s", sl)

	b.Logf("TODO.")

	b.Run("Zap", func(b *testing.B) {
		logger := newZapLogger(zap.DebugLevel)
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(str)
			}
		})
	})

	b.Run("sirupsen/logrus", func(b *testing.B) {
		logger := newLogrus()
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(sl)
			}
		})
	})

	b.Run("rs/zerolog", func(b *testing.B) {
		logger := newZerolog()
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info().Msg(str)
			}
		})
	})

	// b.Run("goweb/log", func(b *testing.B) {
	// 	logger := newGoWebLogger()
	// 	b.ReportAllocs()
	// 	b.ResetTimer()
	// 	b.RunParallel(func(pb *testing.PB) {
	// 		for pb.Next() {
	// 			logger.Info(f)
	// 		}
	// 	})
	// })
}

//////////////////////////////////////////////////////////////////////// BENCHMARKS ////////////////////////////////////////////////////////////////////////
