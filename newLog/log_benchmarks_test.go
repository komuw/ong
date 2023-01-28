package log_test

import (
	stdlibErrors "errors"
	"fmt"
	"io"
	"math/rand"
	"testing"
	"time"

	ongOldlog "github.com/komuw/ong/log"
	"github.com/rs/zerolog"
	"github.com/sirupsen/logrus"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

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

func newOngLogger() ongOldlog.Logger {
	maxMsgs := 50_000
	return ongOldlog.New(
		io.Discard,
		maxMsgs,
	)
}

func getMessage() (ongOldlog.F, []string) {
	type car struct {
		mft  string
		date uint64
	}
	c := car{mft: "Toyota", date: uint64(1994)}
	f := ongOldlog.F{
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

func noOpFunc(f ongOldlog.F) {
	// func used in the `no logger` benchmark.
	_ = f
}

func BenchmarkBestCase(b *testing.B) {
	f, sl := getMessage()
	str := fmt.Sprintf("%s", sl)
	b.Logf("best case") // best-case because ong/oldLog does not log if it is not error level

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

	b.Run("ong/oldLog", func(b *testing.B) {
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

	b.Run("ong/oldLog", func(b *testing.B) {
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

	b.Run("ong/oldLog", func(b *testing.B) {
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
