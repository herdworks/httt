# variable

The `variable` package implements HyperTemplates' template-variable DSL. The
DSL resembles simple JavaScript template-literal expressions, but it is not a
`text/template` language.

```html
<a href="${ page.path }" ht-apply>${ page.title }</a>
```

## Expressions

An expression is either an implicit lookup list or one top-level function
call:

```text
${ page.foo, page.bar, "Placeholder" }
${ get(page.foo, page.bar, "Placeholder") }
${ text(page.foo, "Placeholder") }
${ markdown(page.foo, "# Placeholder") }
${ example(page.foo, page.bar, "Placeholder") }
```

An expression that does not begin with `name(` is an implicit `get`. Every
unquoted argument is a keypath and is resolved through `Getter.Get`. Literal
arguments may be quoted strings, booleans, finite numbers, or `null`.

Nested calls, grouping parentheses, arrays, objects, `undefined`, trailing
commas, and pipe operators are not supported. Complex logic belongs in a
template-data plugin that prepares a directly readable value or in one
template-variable plugin that performs the complete inline transformation.

## Lookup And Fallbacks

Implicit `get`, explicit `get`, `text`, and `markdown` treat their arguments as
an ordered fallback list. They require at least one keypath and permit at most
one final scalar literal fallback.

The first non-nil value wins. False, zero, empty strings, and empty containers
are selected values and do not fall through. If every keypath is missing or
nil and there is no literal fallback, the expression evaluates to nil.

Generic plugins do not apply fallback semantics. They receive every resolved
keypath value or literal argument in source order, including nil values.

## Entry Points

- `ResolveValue(source, data) (any, error)` accepts exactly one `${ ... }`
  implicit or explicit `get` expression containing keypaths only. It returns
  the first non-nil Go value without converting its type. Surrounding text,
  literals, other functions, and pipes are rejected.
- `RenderString(source, data, syntax, funcs...) (string, error)` evaluates each
  expression to a typed value and then interpolates it. Strings remain
  unquoted; every other non-nil value is encoded with `encoding/json`. A nil
  value returns `*MissingValueError`, and JSON encoding failures are returned
  as rendering errors.
- `RenderNodes(source, data, parent, funcs...) ([]*html.Node, error)` calls
  `RenderString` and parses the result with `html.ParseFragment` using `parent`
  as the parsing context.

`text/template` remains an internal Go-function execution mechanism. Its
commands, functions, pipelines, variables, and grouping syntax are not part of
the public DSL.

## Text-Node Functions

The directive renderer registers two functions only while rendering element
text nodes:

- `text(...)` selects its first non-nil argument and HTML-escapes it.
- `markdown(...)` selects its first non-nil argument and renders it as Markdown.

Both use the same fallback rules as `get`. Attribute and CSS contexts do not
register these functions.

`RenderNodes` interprets ordinary string results as HTML fragments. Authors
must use `text(...)` when angle brackets should remain text.

## CSS Values

`SyntaxCSS` recognizes `--ht-var(...)` while preserving the same expression
grammar:

```css
.hero {
    color: --ht-var("theme.color", red);
    content: --ht-var("label(page.title)", "Placeholder");
}
```

The quoted first argument is a variable expression. The optional second CSS
argument is used only when that complete expression evaluates to nil; false,
zero, empty strings, and empty containers do not use it.
