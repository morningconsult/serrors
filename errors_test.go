package serrors_test

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"text/template"

	"github.com/google/go-cmp/cmp"

	"github.com/morningconsult/serrors"
)

type myTestError struct{}

func (me myTestError) Error() string {
	return "Hello"
}

func TestStatusError_Unwrap(t *testing.T) {
	se := serrors.StatusError{
		Status: serrors.InvalidFormat,
		Err:    myTestError{},
	}
	var me myTestError
	if !errors.As(se, &me) {
		t.Errorf("Unable to unwrap and get the myTestError")
	}
}

func TestErrors(t *testing.T) {
	data := []struct {
		name    string
		err     error
		status  serrors.Status
		message string
	}{
		{
			name: "simple",
			err: serrors.StatusError{
				Status: serrors.NotFound,
				Err:    serrors.New("This is a message"),
			},
			status:  serrors.NotFound,
			message: "This is a message",
		},
		{
			name: "missing error",
			err: serrors.StatusError{
				Status: serrors.NotFound,
				Err:    nil,
			},
			status:  serrors.NotFound,
			message: "",
		},
		{
			name: "wrapped",
			err: fmt.Errorf("Wrapped something: %w", serrors.StatusError{
				Status: serrors.InvalidFormat,
				Err:    serrors.New("Original Error"),
			}),
			status:  serrors.InvalidFormat,
			message: "Wrapped something: Original Error",
		},
	}
	for _, v := range data {
		t.Run(v.name, func(t *testing.T) {
			if diff := cmp.Diff(v.err.Error(), v.message); diff != "" {
				t.Errorf("Expected message `%s`, got `%s`", v.message, v.err.Error())
			}
			var se serrors.StatusError
			if errors.As(v.err, &se) {
				if diff := cmp.Diff(se.Status, v.status); diff != "" {
					t.Errorf("Expected code `%d`, got `%d`", v.status, se.Status)
				}
			} else {
				t.Errorf("Should be an serrors.StatusError: %v", v.err)
			}
		})
	}
}

func TestStackErr(t *testing.T) {
	e := serrors.New("new err")
	se := serrors.WithStack(e)
	data := []struct {
		name         string
		formatString string
		expected     string
	}{
		{
			name:         "string",
			formatString: "%s",
			expected:     "new err",
		},
		{
			name:         "quoted",
			formatString: "%q",
			expected:     `"new err"`,
		},
		{
			name:         "value",
			formatString: "%v",
			expected:     "new err",
		},
		{
			name:         "detailed value",
			formatString: "%+v",
			expected:     expectedStackTrace("new err", 86),
		},
	}
	for _, v := range data {
		t.Run(v.name, func(t *testing.T) {
			result := fmt.Sprintf(v.formatString, se)
			if result != v.expected {
				t.Errorf("Expected `%s`, got `%s`", v.expected, result)
			}
		})
	}
	expectedTrace := strings.Split(expectedStackTrace("", 86), "\n")

	actualTrace, err := serrors.Trace(se, serrors.StandardFormat)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(expectedTrace, actualTrace); diff != "" {
		t.Errorf("Expected `%s`, got `%s`", expectedTrace, actualTrace)
	}

	// re-wrap does nothing
	se2 := serrors.WithStack(se)
	for _, v := range data {
		t.Run(v.name, func(t *testing.T) {
			result := fmt.Sprintf(v.formatString, se2)
			if result != v.expected {
				t.Errorf("Expected `%s`, got `%s`", v.expected, result)
			}
		})
	}

	var empty serrors.StackErr
	if empty.Error() != "" {
		t.Errorf("Expected ``, got `%s`", empty.Error())
	}
	if se2.Error() != "new err" {
		t.Errorf("Expected ``, got `%s`", se2.Error())
	}
}

func TestSentinel(t *testing.T) {
	const msg = "This is a constant error"
	const s = serrors.Sentinel(msg)
	if s.Error() != msg {
		t.Errorf("Expected `%s`, got `%s`", msg, s.Error())
	}
}

func TestNew(t *testing.T) {
	err := serrors.New("test message")
	expected := expectedStackTrace("test message", 162)
	result := fmt.Sprintf("%+v", err)
	if expected != result {
		t.Errorf("expected `%s`, got `%s`", expected, result)
	}
}

func TestErrorf(t *testing.T) {
	data := []struct {
		name         string
		formatString string
		values       []interface{}
		expected     string
	}{
		{
			"wrapped non-stack trace error",
			"This is a %s: %w",
			[]interface{}{"error", errors.New("inner error")},
			expectedStackTrace("This is a error: inner error", 199),
		},
		{
			"wrapped stack trace error",
			"This is a %s: %w",
			[]interface{}{"error", serrors.New("inner error")},
			expectedStackTrace("This is a error: inner error", 186),
		},
		{
			"no error",
			"This is a %s",
			[]interface{}{"error"},
			expectedStackTrace("This is a error", 199),
		},
	}
	for _, v := range data {
		// produce the error outside the anon function below so we get the test
		// as the caller and not test.func1
		errOuter := serrors.Errorf(v.formatString, v.values...)
		t.Run(v.name, func(t *testing.T) {
			result := fmt.Sprintf("%+v", errOuter)
			if v.expected != result {
				t.Errorf("expected `%s`, got `%s`", v.expected, result)
			}
		})
	}
}

func TestTrace(t *testing.T) {
	data := []struct {
		name     string
		inErr    error
		expected string
	}{
		{
			"no trace",
			errors.New("error"),
			"",
		},
		{
			"trace",
			serrors.New("error"),
			expectedStackTrace("", 222),
		},
		{
			"wrapped trace",
			fmt.Errorf("outer: %w", serrors.New("inner")),
			expectedStackTrace("", 227),
		},
	}
	for _, v := range data {
		t.Run(v.name, func(t *testing.T) {
			lines, err := serrors.Trace(v.inErr, serrors.StandardFormat)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(v.expected, strings.Join(lines, "\n")); diff != "" {
				t.Error(diff)
			}
		})
	}

	// invalid format
	invalidFormat := template.Must(template.New("standardFormat").Parse("{{.Function}} ({{.File}}:{{.Foobar}})"))
	x := serrors.New("bad")
	lines, err := serrors.Trace(x, invalidFormat)
	if len(lines) != 0 {
		t.Errorf("Expected no lines ,got `%q`", lines)
	}
	expectedErr := `template: standardFormat:1:27: executing "standardFormat" at <.Foobar>: can't evaluate field Foobar in type runtime.Frame`
	var resultErr string
	if err != nil {
		resultErr = err.Error()
	}
	if expectedErr != resultErr {
		t.Errorf("expected `%s`, got `%s`", expectedErr, resultErr)
	}
}

func TestSentinelComparisons(t *testing.T) {
	const s = serrors.Sentinel("This is a constant error")
	err := s
	if err != s {
		t.Errorf("should be the same")
	}
	if !errors.Is(err, s) {
		t.Errorf("should be the same")
	}
	err2 := serrors.Errorf("Wrapping error: %w", s)
	if !errors.Is(err2, s) {
		t.Errorf("should be there")
	}
}

func TestStackErrIs(t *testing.T) {
	err := serrors.New("foo")
	if !errors.Is(err, err) {
		t.Error("oops")
	}
}

func TestErrorPrinting(t *testing.T) {
	err := serrors.New("error message")
	err2 := serrors.Errorf("wrapped %w", err)
	data := []struct {
		name     string
		err      error
		format   string
		expected string
	}{
		{
			name:     "v",
			err:      err,
			format:   "%v",
			expected: `error message`,
		},
		{
			name:     "plus_v",
			err:      err,
			format:   "%+v",
			expected: expectedStackTrace("error message", 283),
		},
		{
			name:     "s",
			err:      err,
			format:   "%s",
			expected: `error message`,
		},
		{
			name:     "q",
			err:      err,
			format:   "%q",
			expected: `"error message"`,
		},
		{
			name:     "proxy_v",
			err:      err2,
			format:   "%v",
			expected: `wrapped error message`,
		},
		{
			name:     "proxy_plus_v",
			err:      err2,
			format:   "%+v",
			expected: expectedStackTrace("wrapped error message", 283),
		},
		{
			name:     "proxy_s",
			err:      err2,
			format:   "%s",
			expected: `wrapped error message`,
		},
		{
			name:     "proxy_q",
			err:      err2,
			format:   "%q",
			expected: `"wrapped error message"`,
		},
	}
	for _, v := range data {
		t.Run(v.name, func(t *testing.T) {
			result := fmt.Sprintf(v.format, v.err)
			if result != v.expected {
				t.Errorf("Expected `%s`, got `%s`", v.expected, result)
			}
		})
	}
}

func TestWithStackNil(t *testing.T) {
	if serrors.WithStack(nil) != nil {
		t.Error("Got non-nil for nil passed to WithStack")
	}
}

// expectedStackTrace formats a stack trace message based on the message and provided line
// for the test, using the actual outside callers of the test. The test has to pass in
// the expected line number, as the error will not have occurred on the same line
// as the call to this function.
func expectedStackTrace(message string, expectedLine int) string {
	pcs := make([]uintptr, 100)
	// start at 2, skipping runtime.Callers and this function
	n := runtime.Callers(2, pcs)
	frames := runtime.CallersFrames(pcs[:n])

	str := strings.Builder{}
	if message != "" {
		fmt.Fprintln(&str, message)
	}

	frame, _ := frames.Next()
	frame.Line = expectedLine
	_ = serrors.StandardFormat.Execute(&str, frame)
	str.WriteByte('\n')

	for {
		frame, more := frames.Next()
		_ = serrors.StandardFormat.Execute(&str, frame)

		if !more {
			break
		}
		str.WriteByte('\n')
	}
	return str.String()
}
