# variable

The `variable` package powers HyperTemplates template variables. A template
variable is a `${ ... }` expression that can appear in an attribute value or
element text node.

Template variables come in two forms:

* Simple variables resolve directly from TemplateData through `get`.
* Computed variables transform bare data identifiers through plugins.

Examples:

```html
<a href="${ page.path }" ht-apply>${ page.title }</a>
```

`href="${ page.path }"` contains a template variable in an attribute value.
`${ page.title }` is a template variable in an element text node.

## Syntax

The default syntax uses `${ ... }` delimiters instead of `{{ ... }}`.

```gotemplate
${ page.title }
```

normalizes to:

```gotemplate
${ get "page.title" }
```

Comma-separated keypaths are fallbacks:

```gotemplate
${ site.title, page.title, "Untitled" }
```

normalizes to:

```gotemplate
${ get "site.title, page.title" "Untitled" }
```

`get` resolves keypaths through any type that implements `Get(string) any` and
returns the first non-nil value. If the last comma-separated argument is a
string literal, `get` uses it as the fallback when all keypaths miss or the
resolved value is an empty string.

## Computed Variables

Computed variables use plugins as transformers. If the first token in a
pipeline is not `get` and not registered in the caller's function map, the
variable package rewrites that first pipeline command as an implicit `get`.

```gotemplate
${ page.title | upper }
```

normalizes to:

```gotemplate
${ get "page.title" | upper }
```

Pipelines may also start with a registered function. In that shape, top-level
bare identifier arguments are data references:

```gotemplate
${ concat page.title site.title "Untitled" }
```

Registered functions and parenthetical expressions are escape hatches into raw
`text/template` syntax. Explicit `get` calls are left alone, so their arguments
must already use the correct `get` syntax:

```gotemplate
${ upper (get "page.title") }
```

Parenthetical expressions do not perform implicit gets. The only built-in
public DSL function is `get`. Built-in `text/template` and `html/template`
functions (`printf`, `len`, `index`, `call`, `print`, `println`, `and`, `or`,
`not`, `eq`, `ne`, `lt`, `le`, `gt`, `ge`, `html`, `js`, `urlquery`) are
outside the DSL unless registered as plugins by the caller. The rewriter
rejects them at parse/validate time at both pipeline head and tail with a
`function "X" not registered` error.

## Entry Points

- `RenderString(source, data, syntax, funcs...) (string, error)` — for
  attribute values and other contexts that need an inline string. Backed by
  `text/template`. Returns the rendered string.
- `RenderNodes(source, data, parent, funcs...) ([]*html.Node, error)` — for
  text-node content. Renders the source as a string via `text/template`, then
  parses the result with `html.ParseFragment` using `parent` as the context.
  There is no auto-escaping: author data containing `<`/`>` is interpreted as
  HTML markup, and funcmap entries that return HTML strings inject real
  elements. The top-level `directives.RenderValues` caller registers two
  built-in text-node funcs in the funcmap: `text` (HTML-escapes its input —
  a thin alias for `text/template.HTMLEscaper`) and `markdown` (renders
  markdown to HTML).

## Missing Variables

If any action in a rendered template variable source resolves to `nil` (and no
fallback is supplied), rendering aborts and returns a `*MissingValueError`.
The error's `Source` field reports the template source that produced the nil;
callers using `errors.As` can detect and inspect it. The
`directives.RenderValues` call site additionally wraps the error with the
surrounding HTML element (tag name, id, class, attribute name) for context.

Empty strings are valid values and render as empty.

Use a final string literal for per-variable fallbacks:

```gotemplate
${ page.title, "Untitled" }
```

The fallback is returned when the value is nil or an empty string.

Pipelines see nil values directly: `${ myfunc page.missing }` calls
`myfunc(nil)`. If `myfunc` returns a non-nil value (including `""`), the
action succeeds. Only a nil final value triggers `MissingValueError`.

## CSS Values

`SyntaxCSS` keeps CSS contexts readable by translating `--ht-value(...)` calls to
`${ ... }` actions before the normal value rewrite runs.

```css
.hero {
    color: --ht-value("theme.color", red);
    padding: --ht-value("theme.pad", 10px);
}
```

The quoted first argument follows the same grammar as `${ ... }`. The optional
second argument is a CSS fallback applied after the expression is evaluated. It
is used when the expression renders no value, and is not used for present
falsey values such as `""`, `0`, or `false`.

## HTML Directive

The `ht-apply` directive marks an element whose attribute values and direct text
nodes should be scanned for template variables.

```html
<a href="${ page.path }" ht-apply>${ page.title }</a>
```

In tidy mode, the directive is removed after rendering.
