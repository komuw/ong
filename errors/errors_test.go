package errors

import (
	stdErrors "errors"
	"fmt"
	"io/fs"
	"os"
	"testing"

	"go.akshayshah.org/attest"
	"go.uber.org/goleak"
)

func hello() error {
	l := 90
	_ = l
	return bar()
}

func bar() error {
	okay := "okay"
	yello := okay
	err := foo()
	blue := yello
	_ = blue
	return err
}

func foo() error {
	return New("error in foo")
}

func prepFile() error {
	kk := "police"
	_ = kk
	if err := open("/tmp/nonExistentFile-akJGdadE.txt"); err != nil {
		return err
	}
	return nil
}

func open(p string) error {
	f, err := os.Open(p)
	if err != nil {
		return Wrap(err)
	}
	defer f.Close()

	return nil
}

func lateWrapping() error {
	return Wrap(hello())
}

type myCustomErr struct{ err error }

func (a myCustomErr) Error() string { return a.err.Error() }

func (a myCustomErr) Unwrap() error {
	// If you remove this method the issue does not reproduce.
	// See; https://github.com/komuw/ong/issues/466
	return a.err
}

func TestMain(m *testing.M) {
	// call flag.Parse() here if TestMain uses flags
	goleak.VerifyTestMain(m)
}

func TestStackError(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		t.Run("errors.New", func(t *testing.T) {
			t.Parallel()

			err := hello()

			sterr, ok := err.(*stackError)
			attest.True(t, ok)
			attest.Equal(t, sterr.Error(), "error in foo")

			stackTrace := sterr.getStackTrace()
			for _, v := range []string{
				"ong/errors/errors_test.go:30",
				"ong/errors/errors_test.go:23",
				"ong/errors/errors_test.go:17",
				"ong/errors/errors_test.go:80",
			} {
				attest.Subsequence(t, stackTrace, v, attest.Sprintf("\n\t%s: not found in stackTrace: %s", v, stackTrace))
			}
		})

		t.Run("errors.Wrap", func(t *testing.T) {
			t.Parallel()

			err := prepFile()

			sterr, ok := err.(*stackError)
			attest.True(t, ok)
			attest.True(t, stdErrors.Is(err, os.ErrNotExist))

			stackTrace := sterr.getStackTrace()
			for _, v := range []string{
				"ong/errors/errors_test.go:45",
				"ong/errors/errors_test.go:36",
				"ong/errors/errors_test.go:100",
			} {
				attest.Subsequence(t, stackTrace, v, attest.Sprintf("\n\t%s: not found in stackTrace: %s", v, stackTrace))
			}
		})

		t.Run("errors.Dwrap", func(t *testing.T) {
			t.Parallel()

			openFile := func(p string) (errp error) {
				defer Dwrap(&errp)

				f, err := os.Open(p)
				if err != nil {
					return err
				}
				defer f.Close()

				return nil
			}

			err := openFile("/tmp/nonExistentFile-akJGdadE.txt")

			sterr, ok := err.(*stackError)
			attest.True(t, ok)
			{
				// Is, As, Unwrap
				var targetErr *fs.PathError
				attest.True(t, stdErrors.Is(err, os.ErrNotExist))
				attest.NotZero(t, stdErrors.Unwrap(err))
				attest.True(t, stdErrors.As(err, &targetErr))
			}

			stackTrace := sterr.getStackTrace()
			for _, v := range []string{
				// The deferred Dwrap frame differs between normal and race builds.
				"ong/errors/errors_test.go:131",
			} {
				attest.Subsequence(t, stackTrace, v, attest.Sprintf("\n\t%s: not found in stackTrace: %s", v, stackTrace))
			}
		})

		t.Run("late wrapping does not affect traces", func(t *testing.T) {
			t.Parallel()

			err := lateWrapping()

			sterr, ok := err.(*stackError)
			attest.True(t, ok)

			stackTrace := sterr.getStackTrace()
			for _, v := range []string{
				"ong/errors/errors_test.go:30",
				"ong/errors/errors_test.go:23",
				"ong/errors/errors_test.go:17",
				"ong/errors/errors_test.go:53",
			} {
				attest.Subsequence(t, stackTrace, v, attest.Sprintf("\n\t%s: not found in stackTrace: %s", v, stackTrace))
			}
		})
	})

	t.Run("formattting", func(t *testing.T) {
		t.Parallel()

		err := hello()

		attest.Equal(t, fmt.Sprintf("%s", err), "error in foo") //nolint:gocritic
		attest.Equal(t, fmt.Sprintf("%q", err), `"error in foo"`)
		attest.Equal(t, fmt.Sprintf("%v", err), "error in foo") //nolint:gocritic

		extendedFormatting := fmt.Sprintf("%+v", err)
		for _, v := range []string{
			"ong/errors/errors_test.go:30",
			"ong/errors/errors_test.go:23",
			"ong/errors/errors_test.go:17",
			"ong/errors/errors_test.go:175",
		} {
			attest.Subsequence(t, extendedFormatting, v, attest.Sprintf("\n\t%s: not found in extendedFormatting: %s", v, extendedFormatting))
		}
	})

	t.Run("errors Is As Unwrap", func(t *testing.T) {
		t.Parallel()

		err := prepFile()
		var targetErr *fs.PathError

		_, ok := err.(*stackError)
		attest.True(t, ok)
		attest.True(t, stdErrors.Is(err, os.ErrNotExist))
		attest.NotZero(t, stdErrors.Unwrap(err))
		attest.True(t, stdErrors.As(err, &targetErr))

		_ = wrap(err, 2) // This is here to quiet golangci-lint which complains that wrap is always called with an argument of 3.
	})

	t.Run("multiple wrapping preserves traces", func(t *testing.T) {
		t.Parallel()

		f := func() (err error) {
			defer Dwrap(&err)

			e1 := New("hey")
			e2 := Wrap(e1)
			e3 := Errorf("fmting: %w", e2)

			return e3
		}

		err := f()
		extendedFormatting := fmt.Sprintf("%+v", err)

		attest.NotZero(t, StackTrace(err))
		attest.Equal(t, err.Error(), "fmting: hey")
		for _, v := range []string{
			"ong/errors/errors_test.go:213",
			"ong/errors/errors_test.go:220",
		} {
			attest.Subsequence(t, extendedFormatting, v, attest.Sprintf("\n\t%s: not found in extendedFormatting: %s", v, extendedFormatting))
		}
	})

	t.Run("issues/466", func(t *testing.T) {
		t.Parallel()

		{ // success
			var err error = myCustomErr{err: New("hey")}
			err = Wrap(err)
			got := fmt.Sprintf("%+#v", err)

			attest.Subsequence(t, got, "hey")
			attest.Subsequence(t, got, "ong/errors/errors_test.go:237")
		}

		{ // nil
			var err error = nil
			err = Wrap(err)
			got := fmt.Sprintf("%+#v", err)

			attest.Subsequence(t, got, "nil")
		}
	})
}

func TestStackTrace(t *testing.T) {
	t.Parallel()

	t.Run("handles nil", func(t *testing.T) {
		t.Parallel()

		var err error = nil
		got := StackTrace(err)
		attest.Equal(t, got, "")
	})

	t.Run("traces", func(t *testing.T) {
		t.Parallel()

		{
			err := New("hello")
			got := StackTrace(err)
			attest.Subsequence(t, got, "ong/errors/errors_test.go:270")
		}
		{
			err := stdErrors.New("hello stdErrors")
			got := StackTrace(err)
			attest.Subsequence(t, got, "")
		}
		{
			e1 := New("hello")
			err := Wrap(e1)

			got := StackTrace(err)
			attest.Subsequence(t, got, "ong/errors/errors_test.go:280")
		}
		{
			e1 := New("hello")
			err := Errorf("yolo: %w", e1)

			got := StackTrace(err)
			attest.Subsequence(t, got, "ong/errors/errors_test.go:287")
		}
		{
			e1 := New("e1")
			e2 := New("e2")
			err := Join(e2, e1)

			got := StackTrace(err)
			attest.Subsequence(t, got, "ong/errors/errors_test.go:295")
		}
	})
}

type contextError struct {
	message string
	err     error
}

func (e *contextError) Error() string {
	return e.message + ": " + e.err.Error()
}

func (e *contextError) Unwrap() error {
	return e.err
}

func TestWrapContract(t *testing.T) {
	t.Parallel()

	t.Run("repeated wrapping", func(t *testing.T) {
		t.Parallel()

		origin := New("failure")
		originTrace := StackTrace(origin)

		wrapped := Wrap(origin)
		attest.True(t, wrapped == origin)

		wrappedAgain := Wrap(wrapped)
		attest.True(t, wrappedAgain == wrapped)

		deferred := wrappedAgain
		Dwrap(&deferred)
		attest.True(t, deferred == wrappedAgain)
		attest.Equal(t, StackTrace(deferred), originTrace)
	})

	t.Run("external wrappers", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name  string
			outer func(error) (*contextError, error)
		}{
			{
				name: "one wrapper",
				outer: func(err error) (*contextError, error) {
					outer := &contextError{message: "outer", err: err}
					return outer, outer
				},
			},
			{
				name: "two wrappers",
				outer: func(err error) (*contextError, error) {
					inner := &contextError{message: "inner", err: err}
					outer := &contextError{message: "outer", err: inner}
					return outer, outer
				},
			},
			{
				name: "fmt wrapper",
				outer: func(err error) (*contextError, error) {
					inner := &contextError{message: "inner", err: err}
					return inner, fmt.Errorf("outer: %w", inner)
				},
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				origin := New("failure")
				originTrace := StackTrace(origin)
				targetErr, outer := test.outer(origin)

				attest.Equal(t, StackTrace(outer), originTrace)

				wrapped := Wrap(outer)
				attest.Equal(t, wrapped.Error(), outer.Error())
				attest.True(t, stdErrors.Unwrap(wrapped) == outer)
				attest.True(t, stdErrors.Is(wrapped, origin))
				attest.Equal(t, StackTrace(wrapped), originTrace)

				var target *contextError
				attest.True(t, stdErrors.As(wrapped, &target))
				attest.True(t, target == targetErr)
				attest.True(t, Wrap(wrapped) == wrapped)
			})
		}
	})

	t.Run("plain error", func(t *testing.T) {
		t.Parallel()

		origin := stdErrors.New("failure")
		wrapped := Wrap(origin)
		trace := StackTrace(wrapped)

		attest.NotZero(t, trace)
		attest.True(t, Wrap(wrapped) == wrapped)
		attest.Equal(t, StackTrace(Wrap(wrapped)), trace)
		attest.True(t, stdErrors.Is(wrapped, origin))
	})

	t.Run("standard multi cause", func(t *testing.T) {
		t.Parallel()

		plain := stdErrors.New("plain")
		origin := New("failure")
		joined := stdErrors.Join(plain, origin)
		wrapped := Wrap(joined)

		attest.True(t, stdErrors.Unwrap(wrapped) == joined)
		attest.True(t, stdErrors.Is(wrapped, plain))
		attest.True(t, stdErrors.Is(wrapped, origin))
		attest.Equal(t, StackTrace(wrapped), StackTrace(origin))
	})

	t.Run("identity", func(t *testing.T) {
		t.Parallel()

		first := New("first")
		second := New("second")

		attest.False(t, stdErrors.Is(first, second))
		attest.True(t, stdErrors.Is(first, first))
	})

	t.Run("formatting", func(t *testing.T) {
		t.Parallel()

		origin := New("failure")
		outer := &contextError{message: "outer", err: origin}
		wrapped := Wrap(outer)
		message := outer.Error()
		trace := StackTrace(origin)

		tests := []struct {
			name   string
			format string
			want   string
		}{
			{name: "string", format: "%s", want: message},
			{name: "value", format: "%v", want: message},
			{name: "quoted", format: "%q", want: fmt.Sprintf("%q", message)},
			{name: "detailed", format: "%+v", want: message + trace},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				attest.Equal(t, fmt.Sprintf(test.format, wrapped), test.want)
			})
		}
	})
}

func TestErrorfContract(t *testing.T) {
	t.Parallel()

	t.Run("wrapped errors", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			make func() (string, []error, []error, error)
		}{
			{
				name: "nested wrapper",
				make: func() (string, []error, []error, error) {
					origin := New("failure")
					inner := &contextError{message: "inner", err: origin}
					outer := fmt.Errorf("outer: %w", inner)
					return StackTrace(origin), []error{origin, inner}, nil, Errorf("operation: %w", outer)
				},
			},
			{
				name: "multiple wrapped errors",
				make: func() (string, []error, []error, error) {
					first := New("first")
					second := New("second")
					return StackTrace(first), []error{first, second}, nil, Errorf("failures: %w; %w", first, second)
				},
			},
			{
				name: "mixed formatting",
				make: func() (string, []error, []error, error) {
					shown := New("shown")
					wrapped := New("wrapped")
					return StackTrace(wrapped), []error{wrapped}, []error{shown}, Errorf("%v: %w", shown, wrapped)
				},
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				wantTrace, wrapped, notWrapped, err := test.make()
				attest.Equal(t, StackTrace(err), wantTrace)
				for _, target := range wrapped {
					attest.True(t, stdErrors.Is(err, target))
				}
				for _, target := range notWrapped {
					attest.False(t, stdErrors.Is(err, target))
				}
				attest.Equal(t, fmt.Sprintf("%+v", err), err.Error()+wantTrace)
			})
		}
	})

	t.Run("does not duplicate a formatted canonical stack", func(t *testing.T) {
		t.Parallel()

		origin := New("failure")
		trace := StackTrace(origin)
		tests := []struct {
			name   string
			format string
			args   []any
		}{
			{name: "formatted wrap", format: "operation: %+w", args: []any{origin}},
			{name: "formatted value and wrap", format: "operation: %+v: %w", args: []any{origin, origin}},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				err := Errorf(test.format, test.args...)
				formatted := fmt.Sprintf("%+v", err)

				attest.Equal(t, StackTrace(err), trace)
				attest.Subsequence(t, formatted, trace)
				attest.Equal(t, formatted, err.Error())
				attest.True(t, stdErrors.Is(err, origin))
			})
		}
	})

	t.Run("new stack", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			make func(error) error
			want string
		}{
			{
				name: "percent v error",
				make: func(err error) error { return Errorf("operation: %v", err) },
				want: "operation: failure",
			},
			{
				name: "no error operand",
				make: func(_ error) error { return Errorf("failure %d", 42) },
				want: "failure 42",
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				origin := New("failure")
				err := test.make(origin)

				attest.Equal(t, err.Error(), test.want)
				attest.False(t, stdErrors.Is(err, origin))
				attest.NotZero(t, StackTrace(err))
				attest.NotEqual(t, StackTrace(err), StackTrace(origin))
			})
		}
	})
}
