// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package errors

import (
	"reflect"
	"testing"
)

// Some of the code here is inspired(or taken from) by:
//   (a) https://github.com/golang/go/blob/go1.20.14/src/errors/join.go whose license(BSD 3-Clause) can be found here: https://github.com/golang/go/blob/go1.20.14/LICENSE

func TestJoinReturnsNil(t *testing.T) {
	if err := Join(); err != nil {
		t.Errorf("errors.Join() = %v, want nil", err)
	}
	if err := Join(nil); err != nil {
		t.Errorf("errors.Join(nil) = %v, want nil", err)
	}
	if err := Join(nil, nil); err != nil {
		t.Errorf("errors.Join(nil, nil) = %v, want nil", err)
	}
}

func TestJoin(t *testing.T) {
	err1 := New("err1")
	// err2 := New("err2")
	for _, test := range []struct {
		errs []error
		want error
	}{
		{
			errs: []error{err1},
			want: err1,
		},
		// {
		// 	errs: []error{err1, err2},
		// 	want: err1,
		// },
		// {
		// 	errs: []error{err2, err1, nil},
		// 	want: err2,
		// },
		// {
		// 	errs: []error{nil, err2, err1},
		// 	want: err2,
		// },
	} {
		got := Join(test.errs...).(interface{ Unwrap() error }).Unwrap()
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("Join(%v) got = %v; want %v", test.errs, got, test.want)
		}
		// if len(got) != cap(got) {
		// 	t.Errorf("Join(%v) returns errors with len=%v, cap=%v; want len==cap", test.errs, len(got), cap(got))
		// }
	}
}

// func TestJoinErrorMethod(t *testing.T) {
// 	err1 := New("err1")
// 	err2 := New("err2")
// 	for _, test := range []struct {
// 		errs []error
// 		want string
// 	}{{
// 		errs: []error{err1},
// 		want: "err1",
// 	}, {
// 		errs: []error{err1, err2},
// 		want: "err1\nerr2",
// 	}, {
// 		errs: []error{err1, nil, err2},
// 		want: "err1\nerr2",
// 	}} {
// 		got := Join(test.errs...).Error()
// 		if got != test.want {
// 			t.Errorf("Join(%v).Error() = %q; want %q", test.errs, got, test.want)
// 		}
// 	}
// }

// func TestJoinStackTrace(t *testing.T) {
// 	t.Parallel()

// 	t.Run("errors.Join", func(t *testing.T) {
// 		t.Parallel()

// 		err1 := New("hello")
// 		err2 := hello()

// 		{
// 			err3 := Join(err1, err2)

// 			sterr, ok := err3.(*joinError)
// 			attest.True(t, ok)
// 			attest.Equal(t, sterr.Error(), "hello\nerror in foo")

// 			stackTrace := sterr.getStackTrace()
// 			for _, v := range []string{
// 				"ong/errors/join_test.go:92", // Join only shows stack trace of first error. ie, err1
// 			} {
// 				attest.Subsequence(t, stackTrace, v, attest.Sprintf("\n\t%s: not found in stackTrace: %s", v, stackTrace))
// 			}
// 		}

// 		{
// 			err3 := Join(err2, err1)

// 			sterr, ok := err3.(*joinError)
// 			attest.True(t, ok)
// 			attest.Equal(t, sterr.Error(), "error in foo\nhello")

// 			stackTrace := sterr.getStackTrace()
// 			for _, v := range []string{
// 				// Join only shows stack trace of first error. ie, err2
// 				"ong/errors/errors_test.go:30",
// 				"ong/errors/errors_test.go:23",
// 				"ong/errors/errors_test.go:17",
// 				"ong/errors/join_test.go:93",
// 			} {
// 				attest.Subsequence(t, stackTrace, v, attest.Sprintf("\n\t%s: not found in stackTrace: %s", v, stackTrace))
// 			}
// 		}
// 	})
// }
