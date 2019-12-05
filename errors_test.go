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

package serrors_test

import (
	"errors"
	"fmt"
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
			expected: `new err
github.com/morningconsult/serrors_test.TestStackErr (github.com/morningconsult/serrors_test/errors_test.go:96)
testing.tRunner (testing/testing.go:909)
runtime.goexit (runtime/asm_amd64.s:1357)`,
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
	expectedTrace := `["github.com/morningconsult/serrors_test.TestStackErr (github.com/morningconsult/serrors_test/errors_test.go:96)" "testing.tRunner (testing/testing.go:909)" "runtime.goexit (runtime/asm_amd64.s:1357)"]`
	out, err := serrors.Trace(se.(serrors.StackTracer), serrors.StandardFormat)
	if err != nil {
		t.Fatal(err)
	}
	actualTrace := fmt.Sprintf("%q", out)
	if expectedTrace != actualTrace {
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
	expected := `test message
github.com/morningconsult/serrors_test.TestNew (github.com/morningconsult/serrors_test/errors_test.go:174)
testing.tRunner (testing/testing.go:909)
runtime.goexit (runtime/asm_amd64.s:1357)`
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
			`This is a error: inner error
github.com/morningconsult/serrors_test.TestErrorf.func1 (github.com/morningconsult/serrors_test/errors_test.go:219)
testing.tRunner (testing/testing.go:909)
runtime.goexit (runtime/asm_amd64.s:1357)`,
		},
		{
			"wrapped stack trace error",
			"This is a %s: %w",
			[]interface{}{"error", serrors.New("inner error")},
			`This is a error: inner error`,
		},
		{
			"no error",
			"This is a %s",
			[]interface{}{"error"},
			`This is a error
github.com/morningconsult/serrors_test.TestErrorf.func1 (github.com/morningconsult/serrors_test/errors_test.go:219)
testing.tRunner (testing/testing.go:909)
runtime.goexit (runtime/asm_amd64.s:1357)`,
		},
	}
	for _, v := range data {
		t.Run(v.name, func(t *testing.T) {
			errOuter := serrors.Errorf(v.formatString, v.values...)
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
		expected []string
	}{
		{
			"no trace",
			errors.New("error"),
			nil,
		},
		{
			"trace",
			serrors.New("error"),
			[]string{
				"github.com/morningconsult/serrors_test.TestTrace (github.com/morningconsult/serrors_test/errors_test.go:241)",
				"testing.tRunner (testing/testing.go:909)",
				"runtime.goexit (runtime/asm_amd64.s:1357)",
			},
		},
		{
			"wrapped trace",
			fmt.Errorf("outer: %w", serrors.New("inner")),
			[]string{
				"github.com/morningconsult/serrors_test.TestTrace (github.com/morningconsult/serrors_test/errors_test.go:250)",
				"testing.tRunner (testing/testing.go:909)",
				"runtime.goexit (runtime/asm_amd64.s:1357)",
			},
		},
	}
	for _, v := range data {
		t.Run(v.name, func(t *testing.T) {
			lines, err := serrors.Trace(v.inErr, serrors.StandardFormat)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(v.expected, lines); diff != "" {
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

func TestNillStackErr(t *testing.T) {
	err := serrors.WithStack(nil)
	if err != nil {
		t.Error("Expected nil error, got", err)
	}
}

func TestStatusStrings(t *testing.T) {
	data := []struct {
		status   serrors.Status
		expected string
	}{
		{
			status:   serrors.Conflict,
			expected: "Conflict",
		},
		{
			status:   serrors.InvalidFormat,
			expected: "InvalidFormat",
		},
		{
			status:   serrors.Forbidden,
			expected: "Forbidden",
		},
		{
			status:   serrors.NotFound,
			expected: "NotFound",
		},
		{
			status:   serrors.Internal,
			expected: "Internal",
		},
		{
			status:   serrors.NoAuth,
			expected: "NoAuth",
		},
		{
			status:   800,
			expected: "Unknown status: 800",
		},
	}
	for _, v := range data {
		if v.status.String() != v.expected {
			t.Errorf("Expected `%s`, got `%s`", v.expected, v.status.String())
		}
	}
}
