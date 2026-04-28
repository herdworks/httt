# value

The `value` package powers HyperTemplates template values. A template value is a
`${ ... }` expression that can appear in an attribute value or element text node.

Template values come in two forms:

* Simple values resolve directly from TemplateData through `httd`.
* Computed values resolve data first, then transform it through plugins.

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
${ httd "page.title" }
```

Comma-separated keypaths are fallbacks:

```gotemplate
${ site.title, page.title | default "Untitled" }
```

normalizes to:

```gotemplate
${ httd "site.title, page.title" | default "Untitled" }
```

`httd` resolves keypaths through any type that implements `Get(string) any` and
returns the first non-nil value.

## Computed Values

Computed values use plugins as downstream transformers. Every supported
pipeline starts with `httd`, either explicitly or through the implicit rewrite.

```gotemplate
${ page.title | upper }
```

normalizes to:

```gotemplate
${ httd "page.title" | upper }
```

The only built-in DSL functions are `httd` and `default`. Built-in
`text/template` functions such as `printf`, `len`, `index`, and `call` are
outside the DSL unless registered as plugins by the caller.

## Missing Values

If any action in a rendered template value source resolves to
`text/template`'s `"<no value>"` marker, rendering discards the partial output
and emits the original user-visible source verbatim. That keeps unresolved
values visible instead of leaking partial substitutions.

Use `default` for per-value fallbacks:

```gotemplate
${ page.title | default "Untitled" }
```

`default` returns the fallback when the value is nil or an empty string.

## CSS Values

`SyntaxCSS` keeps CSS contexts readable by translating `--ht-value(...)` calls to
`${ ... }` actions before the normal value rewrite runs.

```css
.hero {
    color: --ht-value("theme.color", red);
    padding: --ht-value("theme.pad", 10px);
}
```

The optional second argument is a fallback value, following the same convention
as CSS `var(--name, fallback)`.

## HTML Directive

The `ht-apply` directive marks an element whose attribute values and direct text
nodes should be scanned for template values.

```html
<a href="${ page.path }" ht-apply>${ page.title }</a>
```

In tidy mode, the directive is removed after rendering.
