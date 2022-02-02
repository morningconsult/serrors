package serrors_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/morningconsult/serrors"

	"github.com/google/go-cmp/cmp"
)

func TestNewStatusError(t *testing.T) {
	code := 400
	msg := "oh no"
	err := serrors.NewStatusError(code, msg)

	var sc serrors.StatusCoder
	if !errors.As(err, &sc) {
		t.Fatal("err is not a StatusCoder")
	}

	if got := sc.StatusCode(); got != code {
		t.Errorf("want code %d, got %d", code, got)
	}

	if err.Error() != msg {
		t.Errorf("want error %q, got %q", msg, err.Error())
	}
}

func TestNewStatusErrorf(t *testing.T) {
	code := 400
	err := serrors.NewStatusErrorf(code, "foo: %s", "bar")

	var sc serrors.StatusCoder
	if !errors.As(err, &sc) {
		t.Fatal("err is not a StatusCoder")
	}

	if got := sc.StatusCode(); got != code {
		t.Errorf("want code %d, got %d", code, got)
	}

	wantMsg := "foo: bar"
	if msg := err.Error(); msg != wantMsg {
		t.Errorf("want message %q, got %q", wantMsg, msg)
	}
}

func TestNewStatusErrorf_wraps_stack_tracer(t *testing.T) {
	code := 400

	err := serrors.New("oh no")
	err = serrors.NewStatusErrorf(code, "wrapping: %w", err)

	var sc serrors.StatusCoder
	if !errors.As(err, &sc) {
		t.Fatal("err is not a StatusCoder")
	}

	if got := sc.StatusCode(); got != code {
		t.Errorf("want code %d, got %d", code, got)
	}

	wantMsg := "wrapping: oh no"
	if msg := err.Error(); msg != wantMsg {
		t.Errorf("want message %q, got %q", wantMsg, msg)
	}

	wantVerbose := expectedStackTrace("wrapping: oh no", 55)
	got := fmt.Sprintf("%+v", err)
	if diff := cmp.Diff(wantVerbose, got); diff != "" {
		t.Errorf("results differ (-want +got):\n%s", diff)
	}
}

func TestWithStatus(t *testing.T) {
	err := serrors.New("test message")
	err = serrors.WithStatus(200, err)

	expected := expectedStackTrace("test message", 80)
	result := fmt.Sprintf("%+v", err)
	if diff := cmp.Diff(expected, result); diff != "" {
		t.Errorf("results differ (-want +got):\n%s", diff)
	}
}

func TestWithStatus_nil(t *testing.T) {
	defer func() {
		wantPanic := "cannot attach status to nil error"
		if r := recover(); r == nil || r != wantPanic {
			t.Errorf("want panic %q, got %q", wantPanic, r)
		}
	}()
	serrors.WithStatus(200, nil) // nolint: errcheck
}

func TestNewFromStatus(t *testing.T) {
	tests := []struct {
		status   int
		wantText string
	}{
		{http.StatusOK, http.StatusText(http.StatusOK)},
		{http.StatusTeapot, http.StatusText(http.StatusTeapot)},
		{-1, "Unknown Status Error"},
	}

	for _, tt := range tests {
		t.Run(tt.wantText, func(t *testing.T) {
			err := serrors.NewFromStatus(tt.status)
			var sc serrors.StatusCoder
			if !errors.As(err, &sc) {
				t.Fatalf("want status error, got %T", err)
			}

			if sc.StatusCode() != tt.status {
				t.Errorf("want status %d, got %d", tt.status, sc.StatusCode())
			}

			if err.Error() != tt.wantText {
				t.Errorf("want error %q, got %q", tt.wantText, err.Error())
			}

			expected := expectedStackTrace(tt.wantText, 112)
			result := fmt.Sprintf("%+v", err)
			if diff := cmp.Diff(expected, result); diff != "" {
				t.Errorf("results differ (-want +got):\n%s", diff)
			}
		})
	}
}

type messageError struct{}

func (e messageError) Error() string {
	return "foo"
}

func Test_statusError_Format(t *testing.T) {
	err := serrors.WithStatus(400, messageError{})

	want := "foo"
	got := fmt.Sprintf("%s", err)
	if want != got {
		t.Errorf("want message %q, got %q", want, got)
	}
}

func Test_statusError_Is(t *testing.T) {
	err := errors.New("oh boy")
	err400 := serrors.WithStatus(400, err)

	if !errors.Is(err400, err400) {
		t.Error("error does not equal itself")
	}

	err500 := serrors.WithStatus(500, err)
	if errors.Is(err400, err500) {
		t.Error("status codes were not compared")
	}

	wrapped := fmt.Errorf("wrapped: %w", err400)
	if !errors.Is(wrapped, err400) {
		t.Error("not equal to wrapped version")
	}
}
