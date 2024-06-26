package errors

import (
	"fmt"
	"io"
)

// Some of the code here is inspired(or taken from) by:
//   (a) https://github.com/golang/go/blob/go1.20.14/src/errors/join.go whose license(BSD 3-Clause) can be found here: https://github.com/golang/go/blob/go1.20.14/LICENSE

// Join returns an error that wraps the given errors.
// Any nil error values are discarded.
// Join returns nil if every value in errs is nil.
// The error formats as the concatenation of the strings obtained
// by calling the Error method of each element of errs, with a newline
// between each string.
//
// A non-nil error returned by Join implements the Unwrap() error method.
//
// It only returns the stack trace of the first error. Unwrap also only returns the first error.
//
// Note that this function is equivalent to the one in standard library mainly in spirit.
// This is not a direct replacement of the standard library one.
func Join(errs ...error) error {
	n := 0
	for _, err := range errs {
		if err != nil {
			n++
		}
	}
	if n == 0 {
		return nil
	}

	e := &joinError{errs: make([]error, 0, n)}
	for _, err := range errs {
		if err != nil {
			ef := wrap(err, 3)
			e.errs = append(e.errs, ef)
			if e.stackError == nil {
				eff, _ := ef.(*stackError) // ef is guaranteed to be a stackError since it comes from wrap()
				e.stackError = eff
			}
		}
	}

	return e
}

type joinError struct {
	*stackError
	errs []error
}

func (j *joinError) Error() string {
	var b []byte
	for i, err := range j.errs {
		if i > 0 {
			b = append(b, '\n')
		}
		b = append(b, err.Error()...)
	}
	return string(b)
}

func (j *joinError) Unwrap() error {
	if len(j.errs) > 0 {
		return j.errs[0]
	}
	return nil
}

// Format implements the fmt.Formatter interface
func (j *joinError) Format(f fmt.State, verb rune) {
	// todo: this should be kept in sync with `stackError.Format`
	switch verb {
	case 'v':
		if f.Flag('+') {
			_, _ = io.WriteString(f, j.Error())
			_, _ = io.WriteString(f, j.getStackTrace())
			return
		}
		fallthrough
	case 's':
		_, _ = io.WriteString(f, j.Error())
	case 'q':
		_, _ = fmt.Fprintf(f, "%q", j.Error())
	}
}
