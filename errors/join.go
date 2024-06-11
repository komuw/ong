package errors

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
// Note that this function is equivalent to the one in standard library only in spirit.
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

	e := &join{errs: make([]error, 0, n)}
	for _, err := range errs {
		if err != nil {
			ef := wrap(err, 3)
			e.errs = append(e.errs, ef)
			if e.stackError == nil {
				e.stackError = ef
			}
		}
	}

	return e
}

type join struct {
	*stackError
	errs []error
}

func (e *join) Error() string {
	var b []byte
	for i, err := range e.errs {
		if i > 0 {
			b = append(b, '\n')
		}
		b = append(b, err.Error()...)
	}
	return string(b)
}

func (e *join) Unwrap() error {
	if len(e.errs) > 0 {
		return e.errs[0]
	}
	return nil
}
