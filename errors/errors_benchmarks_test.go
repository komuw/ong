package errors

import (
	stdErrors "errors"
	"testing"

	"braces.dev/errtrace"
	pkgErrors "github.com/pkg/errors"
)

var (
	globalStd   error //nolint:gochecknoglobals
	globalOng   error //nolint:gochecknoglobals
	globalPkg   error //nolint:gochecknoglobals
	globalTrace error //nolint:gochecknoglobals
)

func BenchmarkOtherWrappers(b *testing.B) {
	b.Logf("BenchmarkOtherWrappers")

	b.Run("stdError", func(b *testing.B) {
		var err error
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			err = stdErrors.New("error")
		}
		globalStd = err
	})

	b.Run("ongErrors", func(b *testing.B) {
		var err error
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			err = New("error")
		}
		globalOng = err
	})

	b.Run("pkgErrors", func(b *testing.B) {
		var err error
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			err = pkgErrors.New("error")
		}
		globalPkg = err
	})

	b.Run("Errtrace", func(b *testing.B) {
		var err error
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			err = errtrace.New("error")
		}
		globalTrace = err
	})
}

func BenchmarkWrap(b *testing.B) {
	b.Run("first", func(b *testing.B) {
		err := stdErrors.New("error")
		var wrapped error
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			wrapped = Wrap(err)
		}
		globalOng = wrapped
	})

	b.Run("existing", func(b *testing.B) {
		err := New("error")
		var wrapped error
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			wrapped = Wrap(err)
		}
		globalOng = wrapped
	})
}
