# Plugin Pipeline Contract

Plugins are user-defined transformer functions for HyperTemplates computed
values. They are exposed through the `value` package DSL, not as general
`text/template` support.

## Pipeline Shape

Template value pipelines may start with a data lookup or with a registered
plugin function. If the first token in a pipeline is not `get` and not
registered in the caller's function map, the `value` package rewrites that
first command as an implicit `get`.

```gotemplate
${ page.title | upper }
```

normalizes to:

```gotemplate
${ get "page.title" | upper }
```

Plugin-first pipelines use bare identifier arguments where data lookup is
needed:

```gotemplate
${ concat page.title site.title "Untitled" }
```

Explicit `get` calls and parenthetical expressions are raw `text/template`
syntax and are left alone by the rewrite step:

```gotemplate
${ upper (get "page.title") }
```

Parenthetical expressions do not support implicit gets. The only built-in
public DSL function is `get`. Built-in `text/template` functions such as
`printf`, `len`, `index`, `call`, `and`, and `or` are outside the DSL and should
not be available unless they are explicitly registered as plugins under the same
plugin rules.

## Plugin Semantics

Plugins are downstream transformers. A compiled `plugins.Plugin` accepts
ordered arguments. In a `text/template` pipeline, explicit function arguments
come first and the upstream pipeline value is appended last.

```gotemplate
${ page.title | upper }
```

is equivalent to:

```go
upper(get("page.title"))
```

With explicit arguments:

```gotemplate
${ page.updated_at, page.created_at | fmtdate "YYYY-MM-DD" }
```

is equivalent to:

```go
fmtdate("YYYY-MM-DD", get("page.updated_at, page.created_at"))
```

This example shows value flow. If any pipeline step returns `(value, error)`
with a non-nil error, `text/template` aborts execution before the next step
runs.

A registered `plugins.Plugin` uses this Go function shape:

```go
func(...any) (any, error)
```

The `plugins` package adapts plugin functions into a `text/template.FuncMap`.
Callers that bypass the plugin registry and pass a raw `value.FuncMap` may still
use any function signature accepted by `text/template`.

## Values And Errors

`get` returns `any` because HyperTemplates data can contain strings, numbers,
booleans, lists, maps, nested template data, or nil. Plugins own their own input
policy: type validation, type coercion, nil handling, pass-through behavior, and
recoverable fallback behavior.

Plugins should return errors only for unrecoverable render failures. If a value
is missing or recoverable, prefer returning nil, a fallback value, or a
pass-through value with a nil error so the rest of the pipeline can continue.

The final pipeline value may be any Go value. If it reaches template rendering,
`text/template` stringifies it using its normal output rules unless the caller
intercepts the value first.

A `get` string fallback handles nil and empty string values, but it does not
catch plugin errors.

## Reserved Names

`get` and `plugin` are reserved plugin names. Plugin registration should reject
reserved-name collisions before templates are parsed or executed. When plugins
are loaded from an ordered file list, later files with the same resolved plugin
name replace earlier files.

JavaScript plugin files must declare a top-level `const plugin = name` selector.
The selected identifier is the public plugin name and must refer to a top-level
callable binding in the same file.

## Implementation Boundary

The plugin registry belongs to the `plugins` package. The `value` package only
needs the `text/template` function map produced by plugins, and the `scripting`
package only needs to produce Go functions that can be registered as plugins by
callers.
