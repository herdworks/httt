# Template Variable Plugin Contract

Template-variable plugins implement one complete inline transformation for the
HyperTemplates variable DSL. They are not `text/template` pipeline filters.

## Authoring

A plugin is a JavaScript file that default-exports a function:

```js
// upper.js
export default function(input) {
  if (typeof input !== "string") return input
  return input.toUpperCase()
}
```

The runtime is `goja` and supports ECMAScript through ES2020 plus `console`.
There is no Node, npm, `fs`, `fetch`, or module loader. Import declarations are
rejected at load time, `require()` throws at runtime, and top-level `await` is
rejected during transformation.

## Calling Plugins

Call a plugin with one JavaScript-shaped top-level function expression:

```text
${ upper(page.title) }
${ example(page.foo, page.bar, "Placeholder") }
${ fmtdate(page.updated_at, page.created_at, "YYYY-MM-DD") }
```

Every unquoted argument is resolved through `Getter.Get`. Quoted strings,
booleans, and finite numbers are passed as literal values. Plugins
receive all values in source order, including nil values for missing keypaths.
They own their argument validation, coercion, fallback, and pass-through policy.

Nested calls and pipe operators are unsupported. A transformation requiring
composition must be implemented as one plugin or moved into a template-data
plugin so the template can use a simple lookup.

The Go-facing function shape is:

```go
func(...any) (any, error)
```

The `variable` package invokes the function through a private
`text/template.FuncMap` adapter, preserving typed arguments and the typed return
value. A non-nil returned error aborts rendering. A nil result becomes a
`*variable.MissingValueError`; strings interpolate directly and other values
are JSON encoded.

## Loading And Names

`variable.LoadPlugin` loads one JavaScript file. `variable.LoadPlugins` loads
an ordered batch, with later files replacing earlier plugins of the same name.
`variable.MergePlugins` combines plugin groups with the same later-wins rule.

The public name comes from the filename without its extension and must match
`^[A-Za-z_][A-Za-z0-9_]*$`. `get`, `text`, and `markdown` are reserved.

## Errors

Unsupported JavaScript features surface as `*scripting.PluginError`. Plugin
execution errors remain wrapped so callers can inspect them with `errors.Is`
or `errors.As`.
