package serrors

import (
	"bytes"
	"errors"
	"fmt"
	"runtime"
	"text/template"
)

// StackTracer defines an interface that's met by an error that returns a stacktrace. This is
// intended to be used by errors that capture the stacktrace to the source of the error. Each
// invocation of StackTrace() must return a new instance of *runtime.Frames, so that this method
// can be invoked more than once (runtime.Frames uses internal iteration and has no way to reset
// the iterator).
type StackTracer interface {
	StackTrace() *runtime.Frames
}

// stackErr wraps an error with the stack location where the error occurred.
type stackErr struct {
	err         error
	trace       []uintptr
	stackTracer StackTracer
}

// StackTrace returns the call stack frames for the StackErr. If this was the first StackTracer on
// the unwrap chain, it captures when the StackErr was instantiated. If there was an earlier StackTracer,
// the se.stackTracer field is set, and the StackTrace() is returned from it.
//
// A new instance of *runtime.Frames is created every time this method is run, since the struct tracks
// its own offset and cannot be reused.
func (se stackErr) StackTrace() *runtime.Frames {
	if se.stackTracer != nil {
		return se.stackTracer.StackTrace()
	}
	return runtime.CallersFrames(se.trace)
}

// Is implementation to properly handle two StackErr instances being compared to each other using errors.Is.
// Both StackErr instances need to be unwrapped because the trace slice field makes the StackErr not comparable.
func (se stackErr) Is(err error) bool {
	if err, ok := err.(stackErr); ok {
		return errors.Is(se.err, err.err)
	}
	return errors.Is(se.err, err)
}

// WithStack takes in an error and returns an error wrapped in a StackErr with the location where
// an error was first created or returned from third-party code. If there is already an error
// in the error chain that exposes a stacktrace via the StackTrace() method, WithStack returns
// the passed-in error. If a nil error is passed in, nil is returned.
func WithStack(err error) error {
	if err == nil {
		return nil
	}
	var se StackTracer
	if errors.As(err, &se) {
		return err
	}
	return stackErr{
		err:   err,
		trace: buildStackTrace(),
	}
}

func buildStackTrace() []uintptr {
	pc := make([]uintptr, 20)
	n := runtime.Callers(3, pc)
	pc = pc[:n]
	return pc
}

// New builds a StackErr out of a string.
func New(msg string) error {
	return stackErr{
		err:   errors.New(msg),
		trace: buildStackTrace(),
	}
}

// Errorf wraps the error returned by fmt.Errorf in a StackErr. If there is an existing StackTracer
// in the unwrap chain, its stack trace will be preserved.
func Errorf(format string, vals ...interface{}) error {
	err := fmt.Errorf(format, vals...)
	// it's possible that there was already a StackTracer in the unwrap chain in the fmt.Errorf.
	// if so, use that stacktracer in the StackErr.
	var st StackTracer
	if errors.As(err, &st) {
		return stackErr{
			err:         err,
			stackTracer: st,
		}
	}
	return stackErr{
		err:   err,
		trace: buildStackTrace(),
	}
}

// Unwrap exposes the error wrapped by StackErr.
func (se stackErr) Unwrap() error {
	return se.err
}

// Error is the marker interface for an error, it returns the wrapped error or an empty string if there is no
// wrapped error.
func (se stackErr) Error() string {
	if se.err == nil {
		return ""
	}
	return se.err.Error()
}

// StandardFormat is a one-line template used to convert a *runtime.Frame to a
// string. Each entry is formatted as:
//
//	FUNCTION_NAME (FILE_NAME:LINE_NUMBER)
var StandardFormat = template.Must(template.New("standardFormat").Parse("{{.Function}} ({{.File}}:{{.Line}})"))

// PanicFormat is a template resembling the output of a `panic` used to convert
// a *runtime.Frame to a string. Each entry is formatted as:
//
//	FUNCTION_NAME
//		FILE_NAME:LINE_NUMBER
var PanicFormat = template.Must(template.New("standardFormat").Parse("{{.Function}}\n\t{{.File}}:{{.Line}}"))

// Trace returns the stack trace information as a slice of strings formatted using the provided Go template. The valid
// fields in the template are Function, File, and Line. See StandardFormat for an example.
func Trace(e error, t *template.Template) ([]string, error) {
	var se StackTracer
	if !errors.As(e, &se) {
		return nil, nil
	}
	s := make([]string, 0, 20)
	frames := se.StackTrace()
	var b bytes.Buffer
	for {
		b.Reset()
		frame, more := frames.Next()
		err := t.Execute(&b, frame)
		if err != nil {
			return nil, WithStack(err)
		}
		s = append(s, b.String())
		if !more {
			break
		}
	}
	return s, nil
}

// Sentinel is a way to turn a constant string into an error. It allows you to safely declare a
// package-level error so that it can't be accidentally modified to refer to a different value.
type Sentinel string

// Error is the marker interface for an error. This converts a Sentinel into a string for output.
func (s Sentinel) Error() string {
	return string(s)
}
