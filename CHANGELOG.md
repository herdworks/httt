# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/), and this project adheres to [Semantic Versioning](https://semver.org).

## Unreleased

Added CSS-style `--ht-format("...")` surface syntax to the `format` package as an alternative to `${ ... }` for CSS templating contexts.
The CSS form mirrors a CSS custom-property function call and supports an optional `var()`-style fallback: `--ht-format("theme.color", red)`.
CSS syntax is auto-selected in `RenderFormatting` for `<style>` element text content and `style` attributes; the `${ ... }` syntax remains the default everywhere else.

* Added `format.Syntax` type with `SyntaxDefault` and `SyntaxCSS` constants.
* Added a `default` template function for fallback pipelines (`${ page.title | default "Untitled" }`); it returns the default value when the piped value is `nil` or an empty string.
* Changed `format.Render` to accept a trailing `syntax Syntax` parameter and dropped its leading `name string` parameter; the package now hard-codes `"ht-format"` internally since named sub-templates are not part of the DSL.
  _NOTE: this is a breaking API change for direct callers. Drop the first argument and pass `format.SyntaxDefault` to preserve existing behavior._
* Changed the `format` package to expose a minimal public surface: only `Render`, `Getter`, `Syntax`, `SyntaxDefault`, and `SyntaxCSS`. `Parse`, `Validate`, `Rewrite`, the `Template` type, and the `Err*` sentinels are now package-private.
  _NOTE: this is a breaking API change for direct callers of the unexported symbols. `Render` covers the documented use cases._
* Changed `Template.RenderFormatting` to dispatch `SyntaxCSS` for `<style>` element text and `style="..."` attribute values.
* Removed the `text/template` builtin whitelist (`printf`, `print`, `and`, `or`, `len`, `index`, `eq`, `ne`, `lt`, `gt`, `ge`, `le`, `not`, `html`, `js`, `urlquery`, `slice`, `call`) from the reserved-function set; only `httd`, `default`, and user-registered `FuncMap` entries are reserved. Templates that relied on unregistered builtins now fall into the miss-restoration path and render as their original source — register the function via `funcs` to restore previous behavior.
* Changed `format.Render` to detect `text/template`'s `"<no value>"` marker in the rendered output and restore the original template source verbatim instead of leaking the marker. Any action that misses (without a `| default "..."` fallback) now surfaces the user's original template text in the output, making unresolved bindings visible as no-op placeholders rather than mysterious `<no value>` strings. This is all-or-nothing per `Render` call: one miss restores the whole source.

## v0.20.0 (2026-04-21)

Added support for template data array accessors (e.g. `page.attachments.1.href`). 
Added user-generated template data in `ht-block` elements, so `ht-block` elements can access `page.*` template data properties. 
Added the `format` package and `ht-format` rendering support for `${ ... }` text templates backed by `TemplateData.Get()`. 
Improved HyperMark support for detecting inline vs block-level `ht-block` elements and `<!-- more -->` comment detection. 

* Added support for 1-indexed array accessors to `TemplateData.Get()` (e.g. `foo.bar.1.example`).
* Added support for passing a discrete `userData *TemplateData` parameter to `Template.Process()`;
  callers can now seed user-generated template data in `ht-block` elements.
  _NOTE: this is a breaking API change for direct callers of `Template.Process()`._
* Added the `format` package for `${ ... }` text templates with default `httd` lookups and unsupported-action validation.
* Added `ht-format` rendering for formatting element text content with the new `format` package.
* Added `hypermark.HTBlockHTMLParserExtension()` goldmark extension to promote `ht-block` elements to CommonMark type-6 HTML blocks;
  this prevents `ht-block` elements from being wrapped in `<p>` tags when they appear on their own line.
  _NOTE: Supports multi-line opening tags (the `ht-block` attribute can appear on any continuation line of the opening tag). Quoted attribute values are respected, so `ht-block` appearing as a substring inside a value does not trigger promotion. Inline usage of the same tags within prose is unaffected and remains in the surrounding paragraph._
* Changed `<!--more-->` detection to allow for whitespace (e.g. `<!-- more -->`)
* Updated transitive dependencies: `github.com/dlclark/regexp2` v1.7.0 → v1.11.4, `gopkg.in/yaml.v2` v2.3.0 → v2.4.0.

## v0.19.0 (2026-04-17)

* Added `TemplateData.Clone()` for deep-copying nested template data maps and slices without sharing mutable state.
* Added `HyperFiles.Paths()` for returning slash-prefixed file paths from `HyperFiles`.

## v0.18.2 (2026-03-30)

* Added `version.go` with `hypertemplates.LibVersion` and `hypertemplates.LibReleaseDate` properties
* Update `github.com/herdworks/hyperfeed-go` package to v0.19.1

> **NOTE:** until now the `github.com/herdworks/hypertemplates-go` package has attempted to stay in sync with the full suite of HyperTexting packages: 
> 
> * `hypergraph-go`
> * `hyperfeed-go`
> * `hypertemplates-go`
> * `hyperproviders-go`
> * `hypertexting-go`
> * `hyperctl`
> * `libhypertexting`
> * `HyperTextingKit`
> * `hypertexting-app`
> 
> Starting with this release we will decouple the library package versions from the user-facing app versions. 

## v0.18.1 (2026-03-01)

* Fixed a bug in `theme.Archive()` that failed to include the `theme.json` config file in the theme archive.

## v0.18.0 (2026-02-26)

* Added regular expression conditionals for `ht-if` and `ht-not` parameters via the `~=` operator.
* Added `ht-offset` and `ht-limit` attributes for offsetting and limiting `ht-template` iteration output.
* Added `ht-attr` support (including multiple `ht-attr` attributes on a single element) alongside `ht-attrs`.
* Added `Theme.Archive()` and `Theme.FilePaths()` for packaging themes as `.zip` archives.
* Added theme data support via `Theme.TemplateData`, `ThemeConfig.DataDirectory`, and `Theme.DataDir()`.
* Added `ThemeConfig.FragmentsDirectory` and `Theme.FragmentsDir()` with a default `fragments` directory.
* Added markdown word counting helpers and exposed `wordcount` in parsed markdown template data.
* Added `TemplateData.Exists()` for key-path existence checks across scalars, maps, slices, and nil values.
* Added support for "static" page content files (for example `content/**/index.html`).
* Added HTML template data parsing for `.html` inputs (`title`, `description`, `created_at`, and `updated_at`).
* Changed `ht-block` fragment template data to use the `block.*` namespace (including `block.content`).
* Changed include and block attribute forwarding to target the first renderable fragment node without tag-name matching.
* Changed theme archive output to include empty configured theme directories even when they contain no files.
* Changed theme metadata default `Name`/`Version` values.
* Changed theme type names from `HyperTheme` and `HyperThemeConfig` to `Theme` and `ThemeConfig`.
* Changed standard feed parsing to use `github.com/herdworks/hyperfeed-go` (including OPML support).
* Fixed `TemplateData.SetPathValue()` recursion for nested `map[string]any` key-path updates.
* Fixed `ht-template` `ht.index` and `ht.count` property setters when composing nested template data.
* Removed the in-repo `opml` and `atom` packages in favor of `github.com/herdworks/hyperfeed-go`.
* Removed the bundled `bootstrap` package in favor of `github.com/hypertexting-go/bootstrap`.

## v0.17.3 (2025-09-02)

Initial release of the `hypertemplates` package from the standalone `herdworks/hypertemplates-go` repo.

* Completed initial migration from `herdworks/hypertexting-monorepo`
* Adoped `v` prefixed tag naming convention beginning with the `v0.17.3` release

## 0.17.2

See `herdworks/hypertexting-monorepo` for version 0.17.2 and prior changes.
