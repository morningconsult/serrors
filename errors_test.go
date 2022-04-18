package serrors_test

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/morningconsult/serrors"
)

func Test_stackErr(t *testing.T) {
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
			expected:     expectedStackTrace("new err", 16),
		},
	}
	for _, v := range data {
		t.Run(v.name, func(t *testing.T) {
			result := fmt.Sprintf(v.formatString, se)
			if diff := cmp.Diff(v.expected, result); diff != "" {
				t.Errorf("stacktrace differs\n%s", diff)
			}
		})
	}
	expected := expectedStackTrace("", 16)

	actualLines, err := serrors.Trace(se, serrors.PanicFormat)
	if err != nil {
		t.Fatal(err)
	}

	actual := strings.Join(actualLines, "\n")

	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("stacktrace differs:\n%s", diff)
	}

	// re-wrap does nothing
	se2 := serrors.WithStack(se)
	for _, tt := range data {
		t.Run(tt.name, func(t *testing.T) {
			result := fmt.Sprintf(tt.formatString, se2)
			if diff := cmp.Diff(tt.expected, result); diff != "" {
				t.Errorf("stacktrace differs\n%s", diff)
			}
		})
	}
}

func TestSentinel(t *testing.T) {
	const msg = "This is a constant error"
	const s = serrors.Sentinel(msg)
	if s.Error() != msg {
		t.Errorf("want error %q, got %q", msg, s.Error())
	}
}

func TestNew(t *testing.T) {
	err := serrors.New("test message")
	expected := expectedStackTrace("test message", 86)
	result := fmt.Sprintf("%+v", err)
	if diff := cmp.Diff(expected, result); diff != "" {
		t.Errorf("stacktrace differs\n%s", diff)
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
			expectedStackTrace("This is a error: inner error", 123),
		},
		{
			"wrapped stack trace error",
			"This is a %s: %w",
			[]interface{}{"error", serrors.New("inner error")},
			expectedStackTrace("This is a error: inner error", 110),
		},
		{
			"no error",
			"This is a %s",
			[]interface{}{"error"},
			expectedStackTrace("This is a error", 123),
		},
	}
	for _, v := range data {
		// produce the error outside the anon function below so we get the test
		// as the caller and not test.func1
		errOuter := serrors.Errorf(v.formatString, v.values...)
		t.Run(v.name, func(t *testing.T) {
			result := fmt.Sprintf("%+v", errOuter)
			if diff := cmp.Diff(v.expected, result); diff != "" {
				t.Errorf("stacktrace differs\n%s", diff)
			}
		})
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

func Test_stackErr_Is(t *testing.T) {
	err := serrors.New("foo")
	if !errors.Is(err, err) {
		t.Error("oops")
	}

	stdErr := errors.New("bar")
	wrappedErr := serrors.WithStack(stdErr)
	if !errors.Is(wrappedErr, stdErr) {
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
			expected: expectedStackTrace("error message", 162),
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
			expected: expectedStackTrace("wrapped error message", 162),
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
			if diff := cmp.Diff(v.expected, result); diff != "" {
				t.Errorf("stacktrace differs\n%s", diff)
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
	_ = serrors.PanicFormat.Execute(&str, frame)
	str.WriteByte('\n')

	for {
		frame, more := frames.Next()
		_ = serrors.PanicFormat.Execute(&str, frame)

		if !more {
			break
		}
		str.WriteByte('\n')
	}
	return str.String()
}
