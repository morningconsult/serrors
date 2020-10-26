package serrors

import (
	"errors"
	"fmt"
	"io"
)

// StatusCoder represents something that returns a status code.
type StatusCoder interface {
	StatusCode() int
}

type statusError struct {
	HTTPStatusCode int
	Err            error
}

var _ StatusCoder = statusError{}

// NewStatusError returns an error that bundles an HTTP status code. The stack
// trace will be captured.
func NewStatusError(code int, msg string) error {
	return statusError{
		HTTPStatusCode: code,
		Err: &StackErr{
			Err:   errors.New(msg),
			trace: buildStackTrace(),
		},
	}
}

// NewStatusErrorf returns an error that bundles an HTTP status code, sharing
// fmt.Errorf semantics. The stack trace will be captured if not already
// present.
func NewStatusErrorf(code int, format string, a ...interface{}) error {
	err := fmt.Errorf(format, a...)

	// it's possible that there was already a StackTracer in the unwrap chain in the fmt.Errorf.
	// if so, use that stacktracer in the StackErr.
	var st StackTracer
	if errors.As(err, &st) {
		return statusError{
			HTTPStatusCode: code,
			Err: &StackErr{
				Err:         err,
				stackTracer: st,
			},
		}
	}

	return statusError{
		HTTPStatusCode: code,
		Err: &StackErr{
			Err:   err,
			trace: buildStackTrace(),
		},
	}
}

// WithStatus bundles an error with an HTTP status code. The stack trace will
// be captured if not already present.
func WithStatus(code int, err error) error {
	if err == nil {
		panic("cannot attach status to nil error")
	}

	var se StackTracer
	if errors.As(err, &se) {
		return statusError{
			HTTPStatusCode: code,
			Err:            err,
		}
	}

	return statusError{
		HTTPStatusCode: code,
		Err: &StackErr{
			Err:   err,
			trace: buildStackTrace(),
		},
	}
}

// Error returns the underlying error as a string. Status code information
// will not be captured.
func (s statusError) Error() string {
	if s.Err == nil {
		return ""
	}
	return s.Err.Error()
}

// Format uses the underlying formatter for the error, if one exists. This
// allows us to preserve the behavior of the underlying error if the verb
// changes the way it is formatted.
func (s statusError) Format(f fmt.State, verb rune) {
	var formatter fmt.Formatter
	if errors.As(s.Err, &formatter) {
		formatter.Format(f, verb)
		return
	}

	io.WriteString(f, s.Error()) // nolint: errcheck
}

// Unwrap returns the underlying error.
func (s statusError) Unwrap() error {
	return s.Err
}

// StatusCode returns the HTTP status code captured when the error was created.
func (s statusError) StatusCode() int {
	return s.HTTPStatusCode
}
