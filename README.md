[![Go Report Card](https://goreportcard.com/badge/github.com/morningconsult/serrors)](https://goreportcard.com/report/github.com/morningconsult/serrors)
# serrors

serrors is a collection of three error implementations. `StackErr` provides call-stack information,
based on ideas from https://github.com/pkg/errors . `Sentinel` creates immutable package-level
sentinel errors, based on ideas from Dave Cheney. `StatusError` includes error category info alongside
the source error.

## StackErr

One of the biggest differences between errors in Go and exceptions in other languages
is that you don't get a stack trace with a Go error. `serrors.StackErr` fixes this
limitation.

You should never work directly with `serrors.StackErr`. Instead, use one of the three
factory functions.

The `serrors.WithStack` function takes an existing error and wraps it in an
`serrors.StackErr`. This is meant for adapting errors returned by third-party
libraries. If an error is passed to `serrors.WithStack` that already has an error
that implements the `serrors.StackTracer` interface, the error is returned. If a `nil`
error is passed to `serrors.WithStack`, `nil` is returned. These two rules make it 
possible to write the following code and not worry if there's already a stack trace 
(or no error) stored in `err`:

```go
func DoSomething(input string) (string, error) {
    result, err := ThingToCall(input)
    return result, serrors.WithStack(err)
}
```

If you want to wrap an existing error with your own contextual information, use 
`serrors.Errorf`. This works exactly like `fmt.Errorf`, only it wraps the passed-in 
error in an `serrors.StackErr`:

```go
func DoSomething(input string) (string, error) {
    result, err := ThingToCall(input)
    if err != nil {
        err = serrors.Errorf("DoSomething failed on call to ThingToCall: %w", err)
    }
    return result, err
}
```

Like `serrors.WithStack`, there is no additional stack trace added if there's already 
an error that implements the `serrors.StackTracer` interface in the unwrap chain.

If you are creating a new error that's only a `string`, use `serrors.New`. This is a 
drop-in replacement for `errors.New`:

```go
func DoSomething(input string) (string, error) {
    if input == "" {
        return "", serrors.New("cannot supply an empty string to DoSomething")
    }
    result, err := ThingToCall(input)
    return result, serrors.WithStack(err)
}
```

Once you have an error in your unwrap chain with a stack trace, there are two ways
to get the trace back:

- Use the `%+v` formatting directive:

```go
s := serrors.New("This is a stack trace error")
fmt.Printf("%+v\n",s)
```

- Use the `serrors.Trace` function to get a `[]string` that contains each line of
the stack trace:

```go
s := serrors.New("This is a stack trace error")
callStack, err := serrors.Trace(s, serrors.StandardFormat)
fmt.Println(callStack)
```

`serrors.Trace` takes two parameters. The first is the error and the second is a
`text.Template`. There's a default template defined, `serrors.StandardFormat`.
For each line, it produces output that looks like:

```txt
FUNCTION_NAME (FILE_PATH_AND_NAME:LINE_NUMBER)
```

If you want to write your own template, there are three valid variables:

- .Function (for the function name),
- .File (for the file path and name)
- .Line (for the line number).

If you supply an error that doesn't have an `serrors.StackTracer` in its unwrap
chain, `nil` is returned for both the slice of strings and the error. If an invalid
template is supplied, `nil` is returned for the slice and the error is returned
(wrapped in an `serrors.StackErr`). Otherwise, the stack trace is returned as a slice
of strings along with a `nil` error.

Note that by default, the File path will include the absolute path to the file on the
machine that built the code. If you want to hide this path, build using the
`-trimpath` flag.

## Sentinel

This error is based on the error type described by Dave Cheney in his blog post
[Constant Time](https://dave.cheney.net/2019/06/10/constant-time).

For some Go errors, you only need a single value, defined at the package level. These
errors are called _sentinel_ errors, because they have a single value that you are
supposed to check against. One example in the standard library is the `io.EOF` error
that's returned when you get to the end of a file:

```go
var EOF = errors.New("EOF")
```

There is one serious drawback to this declaration: it's a `var`. Technically, it could
be changed by any other package, which would break `==` and `errors.Is` comparisons.
This is one reason why it is considered bad form to use a variable at the package level.

It would be better if you used a `const` to declare a sentinel error. Unfortunately,
you can't use a `const` declaration with `errors.New` because `const` declarations
must be composed of constants, constant literals, and operators, and `errors.New` is
a function. To work around this, `serrors` defines a type called `Sentinel`. This
allows you to write:

```go
const EOF = serrors.Sentinel("EOF")
```

This looks like a function call, but it's actually a (constant) `string` being
typecast to an `serrors.Sentinel` type that meets the error interface. Errors of type
`serrors.Sentinel` work with `==` and `errors.Is`. The following test passes:

```go
func TestSentinel(t *testing.T) {
  const msg = "This is a constant error"
  const s = serrors.Sentinel(msg)
  if s.Error() != msg {
    t.Errorf("Expected `%s`, got `%s`", msg, s.Error())
  }

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
```

Since a sentinel is immutable, it can't contain stack trace information. When you return a sentinel error,
the best practice is to wrap it in an `serrors.WithStack` call so that stack information is captured. Wrapped
sentinel errors must be compared using `errors.Is`.

```go
const NotPositive = serrors.Sentinel("only positive ints are supported")

func DoThing(i int) (int, error) {
  if i <= 0 {
    return 0, serrors.WithStack(NotPositive)
  }
  return i * 2, nil
}
```

The only limitation to sentinels is that the string that's being typecast needs to be a constant expression if you
are assigning it to a `const`. You can, of course, use `serrors.Sentinel` with a `var`, but that provides no additional
functionality over an `errors.New`.

## StatusError

Not all errors are equal; different errors mean different things. Some indicate a bug in the server, while others indicate
bad data being passed in. HTTP status codes are used to indicate this at the web API tier, but there isn't an easy
way to pass this information back to that layer.

The `serrors.StatusError` proves a wrapper to decorate your errors with status information. Rather than include a
dependency on the `net/http` package, it defines its own statuses.

The following statuses are defined in `serrors`:

```go
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
```

By convention, a `Status` with a value of `0` is not valid; it is an error to see one in your program.

An `serrors.StatusError` has two fields, the Status for the error, and the error itself. You can combine the
`serrors.StatusError` with an `serrors.StackErr`:

```go
func DoSomething(input string) (string, error) {
    if input == "" {
        return "", serrors.StackError {
            Status: serrors.InvalidFormat,
            Err: serrors.New("cannot supply an empty string to DoSomething"),
        }
    }
    result, err := ThingToCall(input)
    if err != nil {
        return "", serrors.StackError {
            Status: serrors.Internal,
            Err: serrors.Errorf("DoSomething failed on call to ThingToCall: %w", err)
        }
    }
    return result, nil
}
```

## Testing serrors

The tests for `serrors` require you to run `go test` with the `-trimpath` flag:

```bash
go test -trimpath ./...
``` 
