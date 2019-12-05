// Copyright 2019 The Morning Consult, LLC or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//         https://www.apache.org/licenses/LICENSE-2.0
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package serrors

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
	"text/template"
)

// Status is the type used to represent error types
type Status int

// The error type constants for errors relating to a particular status.
const (
	_ Status = iota
	// InvalidFormat represents badly formatted input errors.
	InvalidFormat
	// Forbidden represents errors where the action is not allowed.
	Forbidden
	// NotFound represents missing or unauthorized data errors.
	NotFound
	// Conflict represents errors from a conflicting application state.
	Conflict
	// Internal represents bugs in the application.
	Internal
	// NoAuth represents errors where the request is missing authentication information
	NoAuth
)

func (s Status) String() string {
	switch s {
	case InvalidFormat:
		return "InvalidFormat"
	case Forbidden:
		return "Forbidden"
	case NotFound:
		return "NotFound"
	case Conflict:
		return "Conflict"
	case Internal:
		return "Internal"
	case NoAuth:
		return "NoAuth"
	default:
		return fmt.Sprintf("Unknown status: %d", s)
	}
}

// StatusError attaches a status to an error. This is used to differentiate between different kinds of failures:
// those caused by badly formatted input, those caused by requests for missing or unauthorized data, and those
// caused by bugs in the code
type StatusError struct {
	Status Status
	Err    error
}

// Error implements the error interface for StatusError
func (se StatusError) Error() string {
	if se.Err == nil {
		return ""
	}
	return se.Err.Error()
}

// Unwrap implements the Wrapper interface for StatusError, allowing it to work with Is and As.
func (se StatusError) Unwrap() error {
	return se.Err
}

// StackTracer defines an interface that's met by an error that returns a stacktrace. This is
// intended to be used by errors that capture the stacktrace to the source of the error. Each
// invocation of StackTrace() must return a new instance of *runtime.Frames, so that this method
// can be invoked more than once (runtime.Frames uses internal iteration and has no way to reset
// the iterator).
type StackTracer interface {
	error
	StackTrace() *runtime.Frames
}

// StackErr wraps an error with the stack location where the error occurred. Use the WithStack
// function to create a StackErr. There can only be one StackErr in the error chain, ideally at
// the root error location.
type StackErr struct {
	Err   error
	trace []uintptr
}

// StackTrace returns the call stack frames captures when the StackErr was instantiated. A new
// instance of *runtime.Frames is created every time this method is run, since the struct tracks
// its own offset and cannot be reused.
func (se StackErr) StackTrace() *runtime.Frames {
	return runtime.CallersFrames(se.trace)
}

// Is implementation to properly handle two StackErr instances being compared to each other using errors.Is.
// Both StackErr instances need to be unwrapped because the trace slice field makes the StackErr not comparable.
func (se StackErr) Is(err error) bool {
	if err, ok := err.(StackErr); ok {
		return errors.Is(se.Err, err.Err)
	}
	return errors.Is(se.Err, err)
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
	out := buildStackErr()
	out.Err = err
	return out
}

func buildStackErr() StackErr {
	pc := make([]uintptr, 20)
	n := runtime.Callers(3, pc)
	pc = pc[:n]
	return StackErr{
		trace: pc,
	}
}

// New builds a StackErr out of a string
func New(msg string) error {
	out := buildStackErr()
	out.Err = errors.New(msg)
	return out
}

// Errorf wraps the error returned by fmt.Errorf in a StackErr if there is no existing StackTracer chained
// within the fmt.Errorf. If there is, the fmt.Errorf is returned directly.
func Errorf(format string, vals ...interface{}) error {
	out := WithStack(fmt.Errorf(format, vals...))
	// it's possible that there was already a stack in an error in the fmt.Errorf.
	// if there wasn't, strip off the top level of the frame pointer, because it will
	// refer to Errorf instead the caller of Errorf.
	if out, ok := (out).(StackErr); ok {
		return StackErr{
			Err:   out.Err,
			trace: out.trace[1:],
		}
	}
	return out
}

// Unwrap exposes the error wrapped by StackErr
func (se StackErr) Unwrap() error {
	return se.Err
}

// Error is the marker interface for an error, it returns the wrapped error or an empty string if there is no
// wrapped error
func (se StackErr) Error() string {
	if se.Err == nil {
		return ""
	}
	return se.Err.Error()
}

// Format controls the optional display of the stack trace. Use %+v to output the stack trace, use %v or %s to output
// the wrapped error only, use %q to get a single-quoted character literal safely escaped with Go syntax for the wrapped
// error.
func (se StackErr) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			fmt.Fprintf(s, "%+v\n", se.Unwrap())
			trace, _ := Trace(se, StandardFormat)
			fmt.Fprintf(s, "%s", strings.Join(trace, "\n"))
			return
		}
		io.WriteString(s, se.Error()) // nolint: errcheck
	case 's':
		io.WriteString(s, se.Error()) // nolint: errcheck
	case 'q':
		fmt.Fprintf(s, "%q", se.Error())
	}
}

// StandardFormat is the default template used to convert a *runtime.Frame to a string. Each entry is formatted as
// "FUNCTION_NAME (FILE_NAME:LINE_NUMBER)"
var StandardFormat = template.Must(template.New("standardFormat").Parse("{{.Function}} ({{.File}}:{{.Line}})"))

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
