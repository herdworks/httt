# format

The `format` package provides a small `text/template` wrapper for HyperTemplates string formatting.

It uses `${ ... }` delimiters instead of `{{ ... }}`.

It rewrites bare keypath actions to `httd` lookups.

`${ page.title }` becomes `${ httd "page.title" }`.

`${ site.title, page.title | printf "%s" }` becomes `${ httd "site.title, page.title" | printf "%s" }`.

`httd` resolves keypaths through any type that implements `Get(string) any`.

Comma-separated keypaths are treated as fallbacks, so `httd` returns the first non-`nil` value.

Templates that start with a reserved function name are left unchanged.

The validator rejects template comments and the `if`, `range`, `break`, `continue`, `template`, `block`, and `with` action keywords.

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