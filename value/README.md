# value

The `value` package powers HyperTemplates template values. A template value is a
`${ ... }` expression that can appear in an attribute value or element text node.

Template values come in two forms:

* Simple values resolve directly from TemplateData through `get`.
* Computed values transform bare data identifiers through plugins.

Examples:

```html
<a href="${ page.path }" ht-apply>${ page.title }</a>
```

`href="${ page.path }"` is a templated attribute value. `${ page.title }` is a
templated element value.

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

## Computed Values

Computed values use plugins as transformers. If the first token in a pipeline is
not `get` and not registered in the caller's function map, the value package
rewrites that first pipeline command as an implicit `get`.

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
  text-node content where the rendered output may include HTML produced by
  funcmap entries returning `html/template.HTML`. Backed by `html/template`
  so user data is auto-escaped while `template.HTML` returns pass through
  raw. The rendered string is parsed as a fragment using `parent` as
  context.

## Missing Values

If any action in a rendered template value source resolves to
`text/template`'s `"<no value>"` marker, rendering discards the partial output
and emits the original user-visible source verbatim. That keeps unresolved
values visible instead of leaking partial substitutions.

Use a final string literal for per-value fallbacks:

```gotemplate
${ page.title, "Untitled" }
```

The fallback is returned when the value is nil or an empty string.

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
nodes should be scanned for template values.

```html
<a href="${ page.path }" ht-apply>${ page.title }</a>
```

In tidy mode, the directive is removed after rendering.
