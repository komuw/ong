package errors

import (
	stdErrors "errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"

	"go.akshayshah.org/attest"
)

func TestJoinReturnsNil(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		errs []error
	}{
		{name: "no errors"},
		{name: "one nil", errs: []error{nil}},
		{name: "two nil", errs: []error{nil, nil}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			attest.Zero(t, Join(test.errs...))
		})
	}
}

func TestJoin(t *testing.T) {
	t.Parallel()

	err1 := New("err1")
	err2 := New("err2")
	tests := []struct {
		name string
		errs []error
		want []error
		text string
	}{
		{
			name: "one error",
			errs: []error{err1},
			want: []error{err1},
			text: "err1",
		},
		{
			name: "two errors",
			errs: []error{err1, err2},
			want: []error{err1, err2},
			text: "err1\nerr2",
		},
		{
			name: "trailing nil",
			errs: []error{err2, err1, nil},
			want: []error{err2, err1},
			text: "err2\nerr1",
		},
		{
			name: "leading nil",
			errs: []error{nil, err2, err1},
			want: []error{err2, err1},
			text: "err2\nerr1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := Join(test.errs...)
			attest.Equal(t, got.Error(), test.text)

			u, ok := got.(interface{ Unwrap() []error })
			attest.True(t, ok)

			causes := u.Unwrap()
			attest.Equal(t, len(causes), len(test.want))
			for i, cause := range causes {
				attest.True(t, cause == test.want[i])
			}
		})
	}
}

func TestJoinSupportsIsAndAs(t *testing.T) {
	t.Parallel()

	sentinel := stdErrors.New("sentinel")
	pathErr := &fs.PathError{Op: "open", Path: "/missing", Err: os.ErrNotExist}
	err := Join(stdErrors.New("other"), sentinel, pathErr)

	attest.True(t, stdErrors.Is(err, sentinel))
	attest.True(t, stdErrors.Is(err, os.ErrNotExist))

	var target *fs.PathError
	attest.True(t, stdErrors.As(err, &target))
	attest.True(t, target == pathErr)
}

func TestJoinStackTrace(t *testing.T) {
	t.Parallel()

	t.Run("preserves branch stacks", func(t *testing.T) {
		t.Parallel()

		err1 := New("first")
		trace1 := StackTrace(err1)
		err2 := New("second")
		trace2 := StackTrace(err2)

		joined := Join(nil, err1, err2)
		attest.Equal(t, StackTrace(joined), trace1)

		causes := joined.(interface{ Unwrap() []error }).Unwrap()
		attest.Equal(t, StackTrace(causes[0]), trace1)
		attest.Equal(t, StackTrace(causes[1]), trace2)

		formatted := fmt.Sprintf("%+v", joined)
		attest.Equal(t, strings.Count(formatted, trace1), 1)
	})

	t.Run("captures stack when branches have none", func(t *testing.T) {
		t.Parallel()

		joined := Join(stdErrors.New("first"), stdErrors.New("second"))
		attest.Subsequence(t, StackTrace(joined), "ong/errors/join_test.go")
	})

	t.Run("uses first available branch stack", func(t *testing.T) {
		t.Parallel()

		plain := stdErrors.New("plain")
		firstStack := New("first stack")
		secondStack := New("second stack")

		joined := Join(plain, firstStack, secondStack)
		attest.Equal(t, StackTrace(joined), StackTrace(firstStack))
	})
}

func TestNestedJoin(t *testing.T) {
	t.Parallel()

	err1 := New("first")
	err2 := New("second")
	err3 := New("third")
	joined := Join(Join(err1, err2), err3)

	for _, target := range []error{err1, err2, err3} {
		attest.True(t, stdErrors.Is(joined, target))
	}
	attest.Equal(t, StackTrace(joined), StackTrace(err1))
}

func TestJoinFormatting(t *testing.T) {
	t.Parallel()

	err := Join(New("first"), New("second"))
	message := "first\nsecond"
	tests := []struct {
		name   string
		format string
		want   string
	}{
		{name: "string", format: "%s", want: message},
		{name: "value", format: "%v", want: message},
		{name: "quoted", format: "%q", want: fmt.Sprintf("%q", message)},
		{name: "detailed", format: "%+v", want: message + StackTrace(err)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			attest.Equal(t, fmt.Sprintf(test.format, err), test.want)
		})
	}

	t.Run("stack Errorf does not panic", func(t *testing.T) {
		t.Parallel()

		errs := []error{
			Errorf("%w", New("err1")),
			Errorf("%w", New("err2")),
		}
		attest.False(t, strings.Contains(fmt.Sprintf("%+#v", Join(errs...)), "PANIC"))
	})
}
