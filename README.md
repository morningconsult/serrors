[![Go Report Card](https://goreportcard.com/badge/github.com/morningconsult/serrors)](https://goreportcard.com/report/github.com/morningconsult/serrors)
# serrors

serrors is a collection of utitilies for creating and working with errors.
These include automatic stack traces based on ideas from
<https://github.com/pkg/errors>, immutable package-level sentinel errors,
based on ideas from Dave Cheney, and the ability to combine HTTP statuses with
errors directly.

## Stack Traces

One of the biggest differences between errors in Go and exceptions in other
languages is that you don't get a stack trace with a Go error. `serrors` fixes
this limitation.

### Creating Stack Traces

The `serrors.WithStack` function takes an existing error and wraps it with a
stack trace. This is meant for adapting errors returned by the standard library
or third-party libraries when you have no other context to provide. If an error
is passed to `serrors.WithStack` that already has an error that implements the
`serrors.StackTracer` interface in its unwrap chain, the passed-in error is
returned. If a `nil` error is passed to `serrors.WithStack`, `nil` is returned.
These two rules make it possible to write the following code and not worry if
there's already a stack trace (or no error) stored in `err`:

```go
func DoSomething(input string) (string, error) {
    result, err := ThingToCall(input)
    return result, serrors.WithStack(err)
}
```

If you want to wrap an existing error with your own contextual information, use
`serrors.Errorf`. This works exactly like `fmt.Errorf`, only it wraps the passed-in
error with a stack trace as well:

```go
func DoSomething(input string) (string, error) {
    result, err := ThingToCall(input)
    if err != nil {
        err = serrors.Errorf("calling ThingToCall: %w", err)
    }
    return result, err
}
```

If there's an error in the unwrap chain that implements the
`serrors.StackTracer` interface, `serrors.Errorf` preserves the existing trace
information.

If you are creating a new error that's only a `string`, use `serrors.New`. This
is a drop-in replacement for `errors.New`:

```go
func DoSomething(input string) (string, error) {
    if input == "" {
        return "", serrors.New("cannot supply an empty string to DoSomething")
    }
    result, err := ThingToCall(input)
    return result, serrors.WithStack(err)
}
```

### Using Stack Traces

Once you have an error in your unwrap chain with a stack trace, there are three
ways to get the trace back:

#### `serrors.Trace`

You can use the `serrors.Trace` function to get a `[]string` that contains each
line of the stack trace:

```go
s := serrors.New("This is a stack trace error")
callStack, err := serrors.Trace(s, serrors.StandardFormat)
fmt.Println(callStack)
```

`serrors.Trace` takes two parameters. The first is the error, and the second is
a `text.Template`. There are two default templates defined.
`serrors.PanicFormat` produces an output that resembles stack traces produced
by the output of a `panic`, while `serrors.StandardFormat` provides a condensed
single-line output.

If you want to write your own template, there are three valid variables:

- .Function (for the function name),
- .File (for the file path and name)
- .Line (for the line number).

If you supply an error that doesn't have an `serrors.StackTracer` in its unwrap
chain, `nil` is returned for both the slice of strings and the error. If an
invalid template is supplied, `nil` is returned for the slice and the error is
returned (wrapped in its own stack trace). Otherwise, the stack trace is
returned as a slice of strings along with a `nil` error.

Note that by default, the file path will include the absolute path to the file
on the machine that built the code. If you want to hide this path, build using
the `-trimpath` flag.

#### `errors.As`

Errors with stack traces produced by `serrors` will implement the
`serrors.StackTracer` interface. You can use `errors.As` to cast the error and
call `StackTrace()` directly:

```go
err := serrors.New("This is a stack trace error")

var stackTracer serrors.StackTracer
if errors.As(err, &stackTracker) {
  frames := stackTracer.StackTrace()
  // ...
}
```

#### Formatting Verb

Errors with stack traces also implement the `fmt.Formatter` interface, so the
`%+v` formatting directive can be used:

```go
err := serrors.New("this is a stack trace error")
fmt.Printf("%+v\n", err)
```

Unfortunately, `fmt` formatting does not unwrap errors, so if an error with a
stack trace is wrapped using a non-`serrors` utility function such as
`fmt.Errorf`, the stack trace will not be printed this way. For that reason,
in situations where it is critical to ensure that the stack trace is recovered,
one of the other two methods should be preferred.

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

## Status Errors

Not all errors are equal; different errors mean different things. Some indicate
a bug in the server, while others indicate bad data being passed in. HTTP
status codes are used to indicate this at the web API tier, but there isn't an
easy way to pass this information back to that layer.

The `serrors` package includes three helpers to attach a status code directly
to an error:

- `serrors.WithStatus` takes an existing and attaches a status code.
- `serrors.NewStatusError` creates a new error from a string.
- `serrors.NewStatusErrorf` creates an error with `fmt.Errorf` semantics.

For convenience, all helpers wrap errors with stack traces as well.

The API allows an `int` to be passed in as a status code. By convention, these
should use HTTP status codes exclusively. Any other status codes run the risk
of violating expectations across application boundaries.

In order to retrieve the status code from an error, you can use `errors.As`:

```go
func HandleRequest(w http.ResponseWriter, r *http.Request) {
    err := ThingToCall()
    var sc serrors.StatusCoder
    switch {
    case errors.As(err, &sc):
        w.WriteHeader(sc.StatusCode())
    case err != nil:
        w.WriteHeader(http.StatusInternalServerError)
    default:
        w.WriteHeader(http.StatusOK)
    }
}
```

## Testing serrors

The tests for `serrors` require you to run `go test` with the `-trimpath` flag:

```bash
go test -trimpath ./...
```
