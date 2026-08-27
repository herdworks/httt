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
${ markdown(page.foo, { "display": "block" }) }
${ example(page.foo, page.bar, "Placeholder") }
${ example(page.foo, { "separator": " · " }) }
```

An expression that does not begin with `name(` is an implicit `get`. Every
unquoted argument is a keypath and is resolved through `Getter.Get`. Literal
arguments may be quoted strings, booleans, or finite numbers.

Nested calls, grouping parentheses, arrays, objects as positional values,
`undefined`, trailing commas, and pipe operators are not supported. A callable
expression may end with one strict JSON object options map. Complex logic
belongs in a template-data plugin that prepares a directly readable value or
in one template-variable plugin that performs the complete inline
transformation.

When a generic function call has a final options map, it receives one argument
with `args` and `options` keys. Calls without options retain positional
arguments. Implicit and explicit `get` expressions do not support options.

## Lookup And Fallbacks

Implicit `get`, explicit `get`, `text`, and `markdown` treat their arguments as
an ordered fallback list. They require at least one keypath and permit at most
one final scalar literal fallback.

The first non-empty template-data value wins. Missing values, typed nils, and
empty strings fall through; false, zero, and empty containers remain selected
values. A final literal, including `""`, is an explicit author-provided value
and is selected literally. If every keypath is missing, nil, or an empty
string and there is no literal fallback, the expression evaluates to nil.

Generic plugins do not apply fallback semantics. They receive every resolved
keypath value or literal argument in source order, including nil values.

## Entry Points

- `ResolveValue(source, data) (any, error)` accepts exactly one `${ ... }`
  implicit or explicit `get` expression containing keypaths only. It returns
  the first non-empty template-data value without converting its type. Surrounding text,
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

- `text(...)` selects its first non-empty template-data argument and HTML-escapes it.
- `markdown(...)` selects its first non-empty template-data argument and renders it as Markdown.

Both use the same fallback rules as `get`. Attribute and CSS contexts do not
register these functions.

`markdown(..., { "display": "block" })` preserves a sole rendered paragraph.
Without that option, Markdown unwraps a sole paragraph for inline use. Other
Markdown options are ignored. `text` and `get` do not support options maps.

`RenderNodes` interprets ordinary string results as HTML fragments. Authors
must use `text(...)` when angle brackets should remain text.

## CSS Values

`SyntaxCSS` recognizes `--ht-var(...)` while preserving the same expression
grammar, including final options maps on callable expressions:

```css
.hero {
    color: --ht-var("theme.color", red);
    content: --ht-var("label(page.title)", "Placeholder");
    accent: --ht-var("label(page.title, { \"tone\": \"bright\" })");
}
```

The quoted first argument is a variable expression. The optional second CSS
argument is used when that complete expression evaluates to nil, including
when all template-data candidates are empty strings; false, zero, and empty
containers do not use it.
