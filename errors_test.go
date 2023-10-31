package serrors_test

import (
	"errors"
	"runtime"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/morningconsult/serrors"
)

func Test_stackErr(t *testing.T) {
	e := serrors.New("new err")
	se := serrors.WithStack(e)
	want := traceLine(15)
	got := traceError(t, se)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("stacktrace differs:\n%s", diff)
	}

	se2 := serrors.WithStack(se)
	if !errors.Is(se2, se) {
		t.Error("wrapping is not a noop")
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
	expected := traceLine(39)
	result := traceError(t, err)
	if diff := cmp.Diff(expected, result); diff != "" {
		t.Errorf("stacktrace differs\n%s", diff)
	}
}

func TestErrorf(t *testing.T) {
	data := []struct {
		name         string
		formatString string
		values       []interface{}
		wantMessage  string
		wantTrace    string
	}{
		{
			"wrapped non-stack trace error",
			"this is an %s: %w",
			[]interface{}{"error", errors.New("inner error")},
			"this is an error: inner error",
			traceLine(80),
		},
		{
			"wrapped stack trace error",
			"this is an %s: %w",
			[]interface{}{"error", serrors.New("inner error")},
			"this is an error: inner error",
			traceLine(65),
		},
		{
			"no error",
			"this is an %s",
			[]interface{}{"error"},
			"this is an error",
			traceLine(80),
		},
	}
	for _, v := range data {
		// produce the error outside the anon function below so we get the test
		// as the caller and not test.func1
		errOuter := serrors.Errorf(v.formatString, v.values...)
		t.Run(v.name, func(t *testing.T) {
			result := traceError(t, errOuter)
			if diff := cmp.Diff(v.wantTrace, result); diff != "" {
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

func TestWithStackNil(t *testing.T) {
	if serrors.WithStack(nil) != nil {
		t.Error("Got non-nil for nil passed to WithStack")
	}
}

// traceLine formats a stack trace message based on the provided line for the
// test, using the actual outside callers of the test.
func traceLine(expectedLine int) string {
	pcs := make([]uintptr, 100)
	// start at 2, skipping runtime.Callers and this function
	n := runtime.Callers(2, pcs)
	frames := runtime.CallersFrames(pcs[:n])

	str := strings.Builder{}
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

// traceError formats a stack trace from an error.
func traceError(t *testing.T, err error) string {
	lines, err := serrors.Trace(err, serrors.PanicFormat)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(lines, "\n")
}
