# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/), and this project adheres to [Semantic Versioning](https://semver.org).

## Unreleased

* Added standard-output JavaScript `console` logging for scripts and plugins, including `console.log`, `console.info`, `console.warn`, `console.error`, `console.debug`, and `console.logger(name)` for named log prefixes.
* Changed JavaScript plugins to regular function files selected with a top-level `const plugin = name`, replacing `ht:plugin` comment metadata, filename fallback names, and function-body plugin scripts.
* Changed scripting-backed plugins to accept ordered variadic arguments so template pipelines can pass explicit function arguments before the upstream pipeline value.
* Renamed the reusable JavaScript script-body APIs from `scripting.Function*` to `scripting.Script*`; `scripting.Function*` now compiles plugin function files.
* Changed the template data lookup function from `httd` to `get`, and relaxed value pipelines so registered functions can start a pipeline when data lookups are written as explicit `get` calls.
* Changed template value fallbacks to use a final string literal argument on `get`, replacing the public `default` function. `--ht-value(...)` can still accept a CSS-friendly second argument, which is applied as the CSS fallback after the expression pipeline.

## v0.22.0 (2026-04-28)

Added new plugins package with formalized HyperTemplates plugin spec, and redesigned HyperTemplates internals around relative paths, replacing the overly complex HyperFile struct with a collection of helper methods that require callers to provider root directories as context for relative path []string slices. 

* Added `ListFiles`, `ListFilesByPattern`, `CleanRelativeFilePath`, `LocalFilePath`, `LocalFilePaths`, and `RelativeFilePath` for file discovery, validation, and root-relative path conversion.
* Added the `plugins` package for scripting-backed plugin loading, metadata, merging, and `text/template` function-map adapters.
* Added validation for unsafe configured theme directory paths.
* Changed theme directory accessors to use side-effect-free `XDirName()` and `XDirPath()` getters instead of mutating config defaults while resolving paths.
* Changed theme asset discovery to populate `Theme.Assets` with paths relative to the theme static directory.
* Changed theme plugin discovery to compile plugins from the theme plugins directory and store them in `Theme.Plugins`.
* Removed the `HyperFile` and `HyperFiles` API in favor of root-scoped path helpers that return sorted, slash-separated relative paths.

## v0.21.1 (2026-04-27)

Added `theme.plugins_dir` config and `Theme.Plugins()` helper for listing installed plugins files.

## v0.21.0 (2026-04-23)

Added a new `ht-apply` templating directive and all-new variable subtitution system with `${ ... }` style template variables.

_NOTE: variable substitution was designed to look and function similar to variable substitution in [Javascript template literals], but the HyperTemplates implementation is **not** based on Javascript._

* Added new template variable declaration, with `${ ... }` delimiter. 
  Template variables can be used in element attributes, _and_ text nodes. 

  ```html
  <title ht-apply>${site.title} | ${page.title}</title>
  <meta name='description' content='${page.description}' ht-apply>
  ```

* Added new `ht-apply` template directive that enables variable substitution on template element attributes and text nodes.

  _NOTE: text node variable substitution is scoped to **direct descendents** of a template element. 
  In the following example, the `${page.greeting.planet}` variable will be replaced, but the `${page.greeting.galaxy}` variable will not. The former is a a direct descendent text node of the parent `<a>` element, but the latter is a direct descendant text node of the nested `<span>` element (a grandchild of the parent `<a>` element)._

  ```html
  <a href='${page.path}' ht-apply>
    Hello, ${page.greeting.planet} world! Welcome to the <span>${page.greeting.galaxy}</span> galaxy.
  </a>
  ```

* Added a CSS template variable declaration, with `--ht-value( ... )` delimiter. 
  
  > NOTE: variable subtitution in `<style>` elements and `style=''` attributes must use the CSS syntax. 
  > 
  > Example:
  > 
  > ```html
  > <style ht-apply>
  >     :root article {
  >         color: --ht-value("page.colors.text, page.colors.body", rgba(0,0,0,0.5));
  >     }
  > </style>
  > ```
  >
  > HyperTemplates CSS variables support two arguments: 
  > 
  > * a comma-separated list of template data keys (see `"page.colors.text, page.colors.body"` above)
  > * a fallback value (see `rgba(0,0,0,0.5)` above), which can be any valid CSS value
  >
  > NOTE: The variable substitution syntax for CSS conforms to the [CSS custom function] specification, 
  > which means they are valid CSS. 
  > Syntax highlighters rejoice!

* Templating variables support functions. The only function that is currently supported is: `default`, for providing fallback values.

  > **Example**
  >
  > ```html
  > <meta name='description' content='${page.description | default "Lorem ipsum, placeholder page description."}' ht-apply>
  > ```


<!-- Links -->
[Javascript template literals]: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Template_literals
[CSS custom function]: https://developer.mozilla.org/en-US/docs/Web/CSS/Guides/Custom_functions_and_mixins/Using_custom_functions#function_basics

## v0.20.0 (2026-04-21)

Added support for template data array accessors (e.g. `page.attachments.1.href`). 
Added user-generated template data in `ht-block` elements, so `ht-block` elements can access `page.*` template data properties. 
Added the `value` package and template value rendering support for `${ ... }` template values backed by `TemplateData.Get()`.
Improved HyperMark support for detecting inline vs block-level `ht-block` elements and `<!-- more -->` comment detection. 

* Added support for 1-indexed array accessors to `TemplateData.Get()` (e.g. `foo.bar.1.example`).
* Added support for passing a discrete `userData *TemplateData` parameter to `Template.Process()`;
  callers can now seed user-generated template data in `ht-block` elements.
  _NOTE: this is a breaking API change for direct callers of `Template.Process()`._
* Added the `value` package for `${ ... }` template values with default `get` lookups and unsupported-action validation.
* Added template value rendering for element text content with the new `value` package.
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
