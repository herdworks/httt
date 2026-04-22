# format

The `format` package provides a small `text/template` wrapper for HyperTemplates string formatting.

It supports two surface syntaxes selected via a `Syntax` argument on `Parse`, `Render`, `Rewrite`, and `Validate`:

* `SyntaxDefault` — `${ ... }` actions (the default).
* `SyntaxCSS` — `--ht-format("...")` or `--ht-format('...')` function-call form, styled as a CSS custom-property function, intended for CSS contexts where `${ ... }` collides with JavaScript template literal conventions.

Both syntaxes share the same action grammar and are backed by the same `httd` keypath lookup.

## Default syntax

It uses `${ ... }` delimiters instead of `{{ ... }}`.

It rewrites bare keypath actions to `httd` lookups.

`${ page.title }` becomes `${ httd "page.title" }`.

`${ site.title, page.title | printf "%s" }` becomes `${ httd "site.title, page.title" | printf "%s" }`.

`httd` resolves keypaths through any type that implements `Get(string) any`.

Comma-separated keypaths are treated as fallbacks, so `httd` returns the first non-`nil` value.

Templates that start with a reserved function name are left unchanged.

The validator rejects template comments and the `if`, `range`, `break`, `continue`, `template`, `block`, and `with` action keywords.

## CSS syntax

In `SyntaxCSS` mode, `--ht-format("<body>")` and `--ht-format('<body>')` calls are translated to `${ <body> }` actions before the default rewrite runs. The quoted argument is the action body — the same grammar as what lives inside `${ ... }`.

Single- and double-quoted strings are accepted and treated identically (CSS makes no distinction). Backslash escapes inside the string: `\\` → `\`, `\"` → `"`, `\'` → `'`. Other `\X` sequences pass through literally.

Examples:

```css
.hero {
    color: --ht-format("theme.color");
    background: --ht-format("theme.bg, site.bg");
    font-family: --ht-format('page.font | printf "%s, sans-serif"');
}
```

### Fallback values

An optional second argument provides a fallback value, mirroring the CSS `var(--name, fallback)` convention. The fallback is emitted when the template body evaluates to `nil` or an empty string.

```css
.hero {
    color: --ht-format("theme.color", red);
    padding: --ht-format("theme.pad", 10px);
    font-family: --ht-format("page.font", "Arial, sans-serif");
    accent: --ht-format("theme.accent", rgb(255, 0, 0));
}
```

The fallback may be any CSS token sequence. Quoted fallbacks have their outer quotes stripped (and the same backslash escapes applied); unquoted fallbacks are captured verbatim up to the balancing `)`, respecting nested parens and quoted strings. The fallback is wired in as a pipeline terminator: `--ht-format("x", red)` normalizes to `${ httd "x" | default "red" }`.

The `default` function is registered in the template func map and may also be used directly in the default `${ ... }` syntax: `${ page.title | default "Untitled" }`.

### Match rules

* The literal `--ht-format(` must be preceded by a non-identifier character (or appear at position 0). `my--ht-format(...)` and `---ht-format(...)` are left untouched.
* Lowercase only — `--HT-FORMAT(...)` and `--Ht-Format(...)` are not matched.
* The first argument must be a single quoted string. Unquoted arguments, multiple first arguments, and empty strings are rejected at parse time.
* The second argument (fallback) is optional. When present, it must be non-empty (`--ht-format("x", )` is rejected).

Reserved keywords (`if`, `range`, `with`, etc.) and template comments are rejected inside `--ht-format(...)` the same way they are inside `${ ... }`.

### Known limitation

`--ht-format(...)` substitution is textual. It will expand even inside a CSS string literal like `content: "--ht-format(\"x\")"`. Avoid that pattern if you need a literal `--ht-format(...)` in your stylesheet output.

## Examples

```html
<html lang='en-US'>
    <head>
        <!-- Global Defaults -->
        <meta ht-include='fragments/head.html' />

        <!-- Layout Metadata, Links, Styles, and Scripts -->
        <title ht-content='${site.title} | ${page.title}'>Placeholder Title</title>
        <meta name='description' content='${page.description,site.description}' ht-attrs='content'>
        <style>
            /* Layout-specific styles */
        </style>
        <script src='/js/components/my-component.js' defer></script>
    </head>
    <body>
        <header>
            <nav>
                <menu>
                    <a href='${link.href}' ht-template='link:data.nav' ht-attrs='href' ht-content='${link.label,link.title}'>Placeholder</a>
                </menu>
            </nav>
        </header>
        <main>
            <article ht-content='${page.content | markdown}'>
                <p>Placeholder content...</p>
            </article>
        </main>
        <footer ht-include='fragments/footer'></footer>
    </body>
</html>
```