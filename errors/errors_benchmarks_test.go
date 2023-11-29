package errors_test

import (
	stdErrors "errors"
	"testing"

	"braces.dev/errtrace"
	ongErrors "github.com/komuw/ong/errors"
	pkgErrors "github.com/pkg/errors"
)

var (
	globalStd   error
	globalOng   error
	globalPkg   error
	globalTrace error
)

func BenchmarkOtherWrappers(b *testing.B) {
	b.Logf("BenchmarkOtherWrappers")

	b.Run("stdError", func(b *testing.B) {
		var err error
		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			err = stdErrors.New("error")
		}
		globalStd = err
	})

	b.Run("ongErrors", func(b *testing.B) {
		var err error
		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			err = ongErrors.New("error")
		}
		globalOng = err
	})

	b.Run("pkgErrors", func(b *testing.B) {
		var err error
		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			err = pkgErrors.New("error")
		}
		globalPkg = err
	})

	b.Run("Errtrace", func(b *testing.B) {
		var err error
		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			err = errtrace.New("error")
		}
		globalTrace = err
	})
}
