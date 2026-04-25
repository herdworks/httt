# Plugin Pipeline Contract

Plugins are user-defined transformer functions for HyperTemplates computed
values. They are exposed through the `value` package DSL, not as general
`text/template` support.

## Pipeline Shape

Every supported value pipeline starts with `httd`, either explicitly or through
the `value` package's implicit rewrite.

```gotemplate
${ page.title | upper }
```

normalizes to:

```gotemplate
${ httd "page.title" | upper }
```

The only built-in DSL functions are `httd` and `default`. Built-in
`text/template` functions such as `printf`, `len`, `index`, `call`, `and`, and
`or` are outside the DSL and should not be available unless they are explicitly
registered as plugins under the same plugin rules.

## Plugin Semantics

Plugins are downstream transformers. A plugin accepts one or more arguments, and
the piped value is always passed as the final argument.

```gotemplate
${ page.title | upper }
```

is equivalent to:

```go
upper(httd("page.title"))
```

```gotemplate
${ page.title | truncate 80 }
```

is equivalent to:

```go
truncate(80, httd("page.title"))
```

These examples show value flow. If any pipeline step returns `(value, error)`
with a non-nil error, `text/template` aborts execution before the next step
runs.

A plugin may return either:

```go
func(...) any
```

or:

```go
func(...) (any, error)
```

## Values And Errors

`httd` returns `any` because HyperTemplates data can contain strings, numbers,
booleans, lists, maps, nested template data, or nil. Plugins own their own input
policy: type validation, type coercion, nil handling, pass-through behavior, and
recoverable fallback behavior.

Plugins should return errors only for unrecoverable render failures. If a value
is missing or recoverable, prefer returning nil, a fallback value, or a
pass-through value with a nil error so the rest of the pipeline can continue.

The final pipeline value may be any Go value. If it reaches template rendering,
`text/template` stringifies it using its normal output rules unless the caller
intercepts the value first.

`default` handles nil and empty string values, but it does not catch plugin
errors.

## Reserved Names

`httd` and `default` are reserved plugin names. Plugin registration should reject
reserved-name collisions and duplicate plugin names before templates are parsed
or executed.

## Implementation Boundary

The plugin registry belongs to the `value` package. The `scripting` package
does not need to know about plugins; it can simply produce Go functions that are
registered as value plugins by callers.
