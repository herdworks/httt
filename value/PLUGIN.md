# Plugin Pipeline Contract

Plugins are user-defined transformer functions for HyperTemplates computed
values. They are exposed through the `value` package DSL, not as general
`text/template` support.

## Authoring a Plugin

A plugin is a JavaScript file that **default-exports a function**. Plugin
authors write idiomatic ES module syntax:

```js
// upper.js
export default function(input) {
  return input.toUpperCase()
}
```

```js
// title.js
export default (input) => input.replace(/\b\w/g, c => c.toUpperCase())
```

The runtime is `goja` (a pure-Go ECMAScript engine). **There is no Node, no
npm, no `fs`, no `fetch`, and no module system.** The features you have
access to:

- The full ECMAScript language up to ES2020 (let/const/classes, arrow
  functions, destructuring, spread, generators, async functions, template
  literals, etc.).
- `console.log` / `info` / `warn` / `error` / `debug` and
  `console.logger(name)` for tagged output.
- The frozen built-in `Math`, `JSON`, `Object`, `Array`, `String`, `Number`,
  `Date`, `RegExp`, etc.

What you don't have:

- `import` declarations or `export ... from` re-exports — the loader
  rejects these at plugin-load time as `PluginError: import is not
  supported in plugins yet`.
- `require()` calls — the wrapper exposes a stubbed `require` that throws
  `PluginError: require is not supported in plugins yet` at runtime.
- Top-level `await` — esbuild rejects this at transform time.

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
`printf`, `len`, `index`, `call`, `and`, and `or` are outside the DSL and
should not be available unless they are explicitly registered as plugins
under the same plugin rules.

## Argument Order

Plugins are downstream transformers. A compiled plugin accepts ordered
arguments. In a `text/template` pipeline, explicit function arguments come
first and the upstream pipeline value is appended last.

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

If any pipeline step returns `(value, error)` with a non-nil error,
`text/template` aborts execution before the next step runs.

A registered plugin uses this Go function shape:

```go
func(...any) (any, error)
```

The `value` package adapts plugin functions into a `text/template.FuncMap`.

## Values And Errors

`get` returns `any` because HyperTemplates data can contain strings,
numbers, booleans, lists, maps, nested template data, or nil. Plugins own
their own input policy: type validation, type coercion, nil handling,
pass-through behavior, and recoverable fallback behavior.

Plugins should return errors only for unrecoverable render failures. If a
value is missing or recoverable, prefer returning nil, a fallback value,
or a pass-through value with a nil error so the rest of the pipeline can
continue.

The final pipeline value may be any Go value. If it reaches template
rendering, `text/template` stringifies it using its normal output rules
unless the caller intercepts the value first.

A `get` string fallback handles nil and empty string values, but it does
not catch plugin errors.

## Reserved Names

`get` is the only reserved plugin name. Plugin registration rejects
reserved-name collisions before templates are parsed or executed. When
plugins are loaded from an ordered file list, later files with the same
resolved plugin name replace earlier files.

The plugin's public name is derived from the file's basename (sans
extension): `upper.js` → `upper`. Names must match
`^[A-Za-z_][A-Za-z0-9_]*$` so they are valid Go identifiers, which
`text/template` requires for `FuncMap` keys.

## Implementation Boundary

Source compilation lives in the `scripting` package: it pre-scans for
unsupported syntax, runs esbuild to transform ESM → CJS, wraps the result
in an IIFE that synthesizes `module`/`exports` and a `require` stub, and
extracts the default-exported function.

The `value` package only needs the `text/template` function map produced
by the loaded plugins.

## Errors As Types

Load-time and runtime errors caused by unsupported features surface as
`*scripting.PluginError`. Callers can detect them with `errors.As`:

```go
var perr *scripting.PluginError
if errors.As(err, &perr) {
    log.Printf("plugin %s used unsupported feature %q", perr.Path, perr.Feature)
}
```
