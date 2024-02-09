package log_test

import (
	"context"
	stdlibErrors "errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"testing"
	"time"

	ongErrors "github.com/komuw/ong/errors"
	"github.com/komuw/ong/log"

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

func newOngLogger() *slog.Logger {
	maxMsgs := 50_000
	return log.New(
		context.Background(),
		io.Discard,
		maxMsgs,
	)
}

func newSlogLogger() *slog.Logger {
	return slog.New(
		slog.NewJSONHandler(io.Discard, nil),
	)
}

func getMessage() ([]string, []any) {
	type car struct {
		mft  string
		date uint64
	}
	c := car{mft: "Toyota", date: uint64(1994)}
	f := map[string]any{
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
		"ongError":       ongErrors.New("This is an ong/errors error"),
	}

	sl := make([]string, 0, len(f))

	for k, v := range f {
		sl = append(sl, k)
		sl = append(sl, fmt.Sprintf("%v", v))
	}

	slAny := []any{}
	for _, v := range sl {
		slAny = append(slAny, v)
	}
	return sl, slAny
}

func noOpFunc(s string) {
	// func used in the `no logger` benchmark.
	_ = s
}

func BenchmarkBestCase(b *testing.B) {
	sl, slAny := getMessage()
	str := fmt.Sprintf("%s", sl)
	b.Logf("best case") // best-case because ong/oldLog does not log if it is not error level

	b.Run("Zap", func(b *testing.B) {
		l := newZapLogger(zap.DebugLevel)
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			l.Info(str)
		}
	})

	b.Run("sirupsen/logrus", func(b *testing.B) {
		l := newLogrus()
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			l.Info(str)
		}
	})

	b.Run("rs/zerolog", func(b *testing.B) {
		l := newZerolog()
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			l.Info().Msg(str)
		}
	})

	b.Run("ong", func(b *testing.B) {
		l := newOngLogger()
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			l.Info(sl[0], slAny...)
		}
	})

	b.Run("slog/json", func(b *testing.B) {
		l := newSlogLogger()
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			l.Info(sl[0], slAny...)
		}
	})

	b.Run("no logger", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			noOpFunc("")
		}
	})
}

func BenchmarkAverageCase(b *testing.B) {
	sl, slAny := getMessage()
	str := fmt.Sprintf("%s", sl)
	logErr := stdlibErrors.New("hey")

	b.Logf("average case")

	b.Run("Zap", func(b *testing.B) {
		l := newZapLogger(zap.DebugLevel)
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			l.Info(str)
			if rand.IntN(100) >= 99 {
				l.Error(logErr.Error())
			}
		}
	})

	b.Run("sirupsen/logrus", func(b *testing.B) {
		l := newLogrus()
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			l.Info(str)
			if rand.IntN(100) >= 99 {
				l.Error(logErr.Error())
			}
		}
	})

	b.Run("rs/zerolog", func(b *testing.B) {
		l := newZerolog()
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			l.Info().Msg(str)
			if rand.IntN(100) >= 99 {
				l.Error().Msg(logErr.Error())
			}
		}
	})

	b.Run("ong", func(b *testing.B) {
		l := newOngLogger()
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			l.Info(sl[0], slAny...)
			if rand.IntN(100) >= 99 {
				l.Error("some-error", logErr)
			}
		}
	})

	b.Run("slog/json", func(b *testing.B) {
		l := newSlogLogger()
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			l.Info(sl[0], slAny...)
			if rand.IntN(100) >= 99 {
				l.Error("some-error", logErr)
			}
		}
	})

	b.Run("no logger", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			noOpFunc("")
		}
	})
}

func BenchmarkWorstCase(b *testing.B) {
	sl, slAny := getMessage()
	str := fmt.Sprintf("%s", sl)
	logErr := stdlibErrors.New("hey")

	b.Logf("worst case")

	b.Run("Zap", func(b *testing.B) {
		l := newZapLogger(zap.DebugLevel)
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			l.Info(str)
			l.Error(logErr.Error())
		}
	})

	b.Run("sirupsen/logrus", func(b *testing.B) {
		l := newLogrus()
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			l.Info(str)
			l.Error(logErr.Error())
		}
	})

	b.Run("rs/zerolog", func(b *testing.B) {
		l := newZerolog()
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			l.Info().Msg(str)
			l.Error().Msg(logErr.Error())
		}
	})

	b.Run("ong", func(b *testing.B) {
		l := newOngLogger()
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			l.Info(sl[0], slAny...)
			l.Error("some-error", logErr)
		}
	})

	b.Run("slog/json", func(b *testing.B) {
		l := newSlogLogger()
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			l.Info(sl[0], slAny...)
			l.Error("some-error", logErr)
		}
	})

	b.Run("no logger", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			noOpFunc("")
		}
	})
}
