# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/), and this project adheres to [Semantic Versioning](https://semver.org).

## Unreleased

* Added repeatable `ht-with="${ variable } as binding"` modifiers for local,
  source-ordered bindings on `ht-apply` and `ht-each`. Remaining `ht-with`
  modifiers trigger an implicit `ht-apply` on their element.
* Added `TemplateData.ShallowClone` for writable, top-level-local template-data
  scopes and `document.AddAttribute` for preserving repeated HTML attributes.
* Changed `document.ForwardAttributes` to accept `appendingKeys`
  before its skipped keys, preserving repeated attributes such as `ht-with` in
  source order.
* Renamed `DirectiveExpression` to `ExpressiveDirective` 
* Removed unused incremental-expression APIs: `IncrementalDirectiveExpression`,
  `TokenizeNext`, and `ConditionalDirective.ParseNextExpression`.

## v0.33.1 (2026-07-27)

* Updated the GitHub Actions Go toolchain to v1.26.2 so benchmark tests using `testing.B.Loop` compile in CI.

## v0.33.0 (2026-07-27)

Major redesign of the HT templating directives. All directives that provide access to template data now use the ${ ... } variable syntax. Directives that were obviated by template variable substitution have been removed to improve overall template consistency. Conditional directives have been consolidated into a single more expressive ht-if directive w/ support for multi-variable expressions (e.g. '${ ... } == ${ ... }'). Every directive can now define its own syntax within a shared grammar – the common denominator is now the template variable, not an obtuse `<target>:<keypath[,keypath]>==<condition>` syntax. Most of the new directives use a new/shared token-based parser with support for template variables, operators, separators, and literal values.

This release drops support for `ht-not`, `ht-attr`, `ht-content`, `ht-query`, and `ht-param`. The `ht-template` directive has been replaced with a new `ht-each` directive. A new `hyperctl theme migrate` command has been provided for semi-automated migration of legacy templating directives, and build logs now produce more helpful error messages to help troubleshoot template directive syntax errors.

* **BREAKING:** Moved the template runtime to the `files`, `theme`, `document`, and `directives` packages, and renamed the `value` package to `variable`. The former root-level runtime compatibility APIs have been removed; callers should use the focused package APIs and `directives.Process`.
* **BREAKING:** Reworked template-variable syntax around explicit `${ ... }` expressions, ordered fallbacks, and one top-level built-in or plugin call. Pipelines, raw `text/template` commands, nesting, grouping, and bare `null` are no longer supported.
* **BREAKING:** Renamed CSS variable interpolation from `--ht-value(...)` to `--ht-var(...)`; the former spelling is no longer recognized.
* **BREAKING:** Redesigned conditional and loop directives. `ht-if` now uses typed presence expressions and explicit comparisons; `ht-each` replaces `ht-template` with typed sources, literal windows, and `each.index` / `each.count` / `each.total` loop metadata. Runtime `ht-not` and `ht-template` have been removed and fail with migration hints.
* **BREAKING:** Replaced legacy `ht-attrs` declarations with repeatable map-valued `${ ... }` attributes; later maps win and directive-valued keys are skipped. Runtime `ht-attr` has been removed. `ht-content`, `ht-param`, and `ht-query` are likewise migration-only; use `ht-apply` and explicit template output.
* **BREAKING:** `ht-base` now requires repeatable `<target> on ${ variable }` declarations, and `ht-pipe` requires `to "<selector>"` with an optional `as <type>` clause. Pipe destinations are quoted literal configuration and repeated declarations are source-ordered fallbacks.
* **BREAKING:** Replaced exported directive attribute lists and grammar APIs with directive-owned discovery and parsing methods. Renamed `variable.Merge` to `variable.MergePlugins`, directive rendering helpers, and `document.MarkdownValueFunc`; removed the old exports.
* Added new `hyperctl theme migrate` command for migrating themes with legacy templating directives.
* Improved directive processing and diagnostics: includes expand before same-element loops, active directives require explicit template-data syntax, and errors identify the authored directive or rendered location. `ht-debug` now accepts only repeated `element` or complete `${ ... }` values, while `ht-block` runs only in the final top-level pass with restricted caller data.

## v0.32.0 (2026-07-01)

* Added `TemplateDataKeyPath` as the branch/leaf representation for TemplateData mutation paths.
* Added `TemplateData.Add`, `TemplateData.Replace`, and `TemplateData.Append` mutation helpers for dot-notation TemplateData paths.
* Changed mutation helpers to share key-path validation. `Set`, `Add`, `Replace`, and `Append` now reject empty path segments and computed function-style accessors such as `length()`, `count()`, and `keys()`.
* Changed `TemplateData.Append` to support a fixed JSON-like slice set (`[]any`, `[]string`, `[]bool`, `[]int`, `[]int64`, `[]float64`, and `[]float32`) without reflection.

## v0.31.0 (2026-06-06)

* Updated github.com/herdworks/hyperfeed-go to version 0.22.0

## v0.30.0 (2026-05-28)

* **BREAKING:** Changed exported logging/debug output APIs to accept explicit `io.Writer` arguments. Affected APIs include `Render`, `Theme.Parse`, `Theme.Archive`, `Template.Process`, `Template.EvaluateElement`, `Template.RenderTemplate`, `Template.RenderBasedElement`, `Template.ResolveBasedAttributes`, `Template.RenderBlock`, `Template.RenderQueryParams`, `Template.Debug`, and `Parameter.Evaluate`. Passing nil preserves previous stdout behavior; non-nil writers are used directly.
* Changed template debug output, conditional regex warnings, base-URL resolution debug output, URL query debug output, and theme archive progress output to write to the supplied writer instead of printing directly to stdout.
* Removed an unstructured stdout print from unknown mention-kind parsing.

## v0.29.0 (2026-05-11)

Changed the value package DSL to return errors on missing data, and improved HTML variable parsing; added CSV/TSV data file support, and implicit `ht-apply` on `ht-block` fragments.

**BREAKING:** value DSL actions whose final value is `nil` now return `*value.MissingValueError` instead of restoring the literal `${...}` source — add a fallback (`${ page.foo, "default" }` or `--ht-value("page.foo", "red")`) to opt out. Empty strings remain valid values and render as empty; errors are wrapped at `Template.RenderValues` call sites with the surrounding HTML element (tag name, id, class, attribute) and callers can detect via `errors.As(err, &miss)` and read `miss.Source` for the failing template source. Separately, `value.RenderNodes` now uses `text/template` and parses output with `html.ParseFragment`; `html/template` is no longer used, so author data with literal `<`/`>` is interpreted as HTML markup — there is no auto-escape in text nodes. Authors who need literal angle brackets must encode them in their data, or use the new `text` value func.

* Added a `text` value func in text-node contexts. Thin alias for `text/template`'s built-in `html` predefined function (`text/template.HTMLEscaper`); HTML-escapes its input. `text(nil)` renders the literal escaped string `&lt;no value&gt;`; use bare `${ page.foo }` when a missing keypath should error instead.
* Added CSV and TSV data file support via a new `csvdata` package. `csvdata.UnmarshalTemplateDataSet` returns a `data.TemplateDataSet` (slice-shaped, natural for record data); `csvdata.UnmarshalTemplateData` wraps that slice as `{"rows": TemplateDataSet{...}}` to satisfy `parse.Unmarshal`'s map-only contract. Options: `WithDelimiter(rune)` (defaults to `,`; pass `'\t'` for TSV), `WithRowsKey(string)` (defaults to `"rows"`).
* Added `.csv` and `.tsv` to `parse.Unmarshal`'s extension dispatcher. `.tsv` uses tab as its delimiter; both produce `{"rows": TemplateDataSet{...}}` shapes.
* Added CSV header-row validation: empty or whitespace-only header cells error with a hint that the row looks like values, not headers; duplicate headers (after `strings.TrimSpace`) error with both column numbers. Headers themselves are stored unmodified as map keys. Cell values are always strings (no type inference; matches `encoding/csv` defaults).
* Added UTF-8 BOM stripping at the start of CSV/TSV files so Excel-exported CSVs do not produce a corrupted first header key.
* Added "implicit ht-apply" on ht-block fragment roots — `${ ... }` expressions in a block template's root element attributes and direct text now resolve without an explicit `ht-apply` attribute. Mirrors the v0.26.1 behavior for `ht-template` elements; descendants still require their own `ht-apply`.
* Changed pipeline semantics so `nil` flows mid-pipeline (e.g. `${ myfunc page.foo }` calls `myfunc` with `nil`); only a nil final value errors.
* Changed the `markdown` value func to return a plain `string` (previously `html/template.HTML`). Existing templates using `${ page.body | markdown }` are unaffected.
* Removed the `html` value func. Bare `${ page.body }` where `page.body` contains HTML markup now inserts real elements; the explicit `| html` filter is no longer needed. To render literal `<`/`>` as text instead of markup, use `| text`.

## v0.28.0 (2026-05-08)

Standardized scripting plugins on ESM default exports.

**BREAKING:** replaced the `Entrypoint`-based plugin authoring
contract (`const plugin = …`, `function namespace()`) with an idiomatic ES
module default-export contract: every plugin file declares
`export default <function>`. Plugin sources are pre-scanned for
unsupported syntax, transformed via `esbuild` (Format=IIFE,
GlobalName=`__pluginMod`), and wrapped in an outer IIFE that injects
optional bindings as top-level locals, defines a `require` stub, and
extracts `__pluginMod.default` after the inner IIFE returns. Existing
plugin files must be migrated. CJS-style `module.exports = …` authoring
is not supported (esbuild rejects it in IIFE format).

* Added `github.com/evanw/esbuild` v0.28.0 as a direct dependency.
  Pinned in `go.mod` because the CJS output shape has shifted across major
  versions and the wrapper's default-export extraction depends on it.
* Added `scripting.PluginError` typed error for unsupported plugin features
  (`import` declarations rejected at load time; runtime `require()` calls
  rejected by the wrapper stub). Both surface through `errors.As`.
* Added `scripting/transform.go` (pre-scan + esbuild Transform) and
  `scripting/plugin_error.go`.
* Replaced 3 binding-shape wrapper variants + 2 plugin wrapper sources +
  4 cache helpers + 2 evaluation paths with a single
  `pluginWrapperSource` + `programForPlugin` + `evalPluginProgram` per
  plugin. The script/inline-eval paths (`Eval*`, `Script*`) are unchanged
  and continue to use `bindingShape`.
* Removed `scripting.Plugin.Entrypoint` field, `Compile()` /
  `CompileContext()` / `CompileTimeout()` / `Run(ctx)` / `RunTimeout()`.
  New surface: `Plugin.Compile(bindingsKeys []string)` returns a
  `PluginCallable` (`func(ctx, bindings, args...) (any, error)`).
* Removed `Engine.NamedFunction*`, `Engine.SeededFunction*` and their
  `*Context` / `*Timeout` variants. The `Engine.Function` /
  `FunctionContext` / `FunctionTimeout` public surface is retained but
  rewritten to use the new transform pipeline; sources must be ESM-shaped.
* Removed `value.PluginEntrypoint` and `data.PluginEntrypoint` constants.
  Reserved plugin names: `get` (was `get`, `plugin`). Reserved namespace
  names: `data` (was `namespace`, `data`).

## 0.27.0 (2026-05-06)

Added support for new embedded/builtin template fragments, and simplified the "plugins" implementation.

Add a new RenderBuiltins step to the template.Process() procedure, with a new `head_start.html` fragment that prepends the `<head>` element. Callers can override the embedded filesystem with their own resources, and they can configure the default builtins.

Unified user-facing and internal meaning of plugins by consolidating `hypertemplates-go/plugin` into `hypertemplates-go/scripting` and adding support for plugin Kinds.

* Unified the user-facing and internal meaning of "plugin": a JavaScript file with a named entrypoint and optional local bindings, with concerns split across `scripting` (runtime), `value` (variable-substitution plugins), and `data` (computed-namespace plugins). The `hypertemplates-go/plugins` package is gone.
* Added `scripting.Plugin` (`Name`, `Path`, `Source`, `Entrypoint`, `Bindings`) with `Compile()` for reusable callables (variable-substitution shape) and `Run(ctx)` for one-shot evaluation with bindings (computed-namespace shape).
* Added `scripting.LoadPlugin` / `LoadPlugins` for reading `.js` files into unbound `Plugin` values; callers set `Entrypoint` and `Bindings`.
* Added `value.Plugin` / `value.Plugins` / `value.LoadPlugin` / `value.LoadPlugins` / `value.Merge`. Reserved names: `get`, `plugin`. Filenames must match `^[A-Za-z_][A-Za-z0-9_]*$`.
* Added `data.Plugin` / `data.LoadPlugin` / `data.NamespaceSegmentsFromRelPath`. Reserved names: `namespace`, `data`. Namespace segments accept `^[A-Za-z0-9_]+$` (leading digits allowed).
* Removed the package-level `scripting.Function` / `NamedFunction` / `SeededFunction` and their `*Context` / `*Timeout` variants. Use `scripting.Plugin{Source, Entrypoint, ...}.Compile()` or `.Run(ctx)` instead. The `*Engine` methods are retained as internal building blocks; `Eval` / `EvalString` / `Script*` are unchanged.
* Changed `Template.Plugins` and `Theme.Plugins` from `plugins.Plugins` to `value.Plugins`. `Theme.DiscoverPlugins` and equivalent helpers in consumer packages call `value.LoadPlugins` directly.
* Added built-in embedded template fragments and a generic embedded-FS fallback for theme resource lookups. The library ships a small set of layout slots that themes can override on disk or disable with an empty file.
* Added `Builtin` struct (`Name`, `Selector`, `Mode`, `Fragment`) describing a layout-injection slot. Modes are `prepend`, `append`, `before`, `after`.
* Added `DefaultBuiltins []Builtin` package-level registry with a `head_start` builtin (prepend into `head`). Callers may append to or replace entries.
* Added `BuiltinsFS fs.FS` package-level variable that defaults to the embedded fragments shipped with the package. Caller programs (e.g. `hypertexting-go`) may replace it with any `fs.FS` to inject their own built-in resource pack.
* Added `Template.RenderBuiltins()` pipeline pass that runs after `ResolveIncludes` and before all directive evaluation; injected builtins flow through the rest of the pipeline naturally (`${...}`, `ht-if`, `ht-attrs`, etc., apply to them). Skipped when `BlockMode` is true.
* Added `MustSubFS(fsys, dir)` helper for callers that ship their own `embed.FS` and want to assign it to `BuiltinsFS`.
* Changed `Template.IncludeFragments` and `Theme.ResourceHTML` to load missing resources from `BuiltinsFS`; empty theme files still short-circuit (opt-out), and the fallback is only consulted on `os.IsNotExist`.

## v0.26.1 (2026-05-05)

Added support for "implicit ht-apply" on ht-template elements.

## v0.26.0 (2026-05-05)

Added `scripting.SetConsoleOutput(io.Writer)` — redirects the writer used by `console.log`/`info`/`warn`/`error`/`debug` and any `console.logger(name)` instance. Pass `io.Discard` to silence script output (useful when a tool consumes stdout, e.g. piping JSON output to `jq`), `os.Stderr` to route diagnostics off stdout, or `nil` to restore the default (resolves `os.Stdout` lazily at write time so tests and callers that swap `os.Stdout` continue to work).

## v0.25.0 (2026-05-05)

This release removes "magic" naming logic from plugins files and OPML data, generalizes the scripting engine's plugin entry point, and adds a generic seeded-runtime primitive that powers computed template data in `hypertexting-go`.

* Added `Engine.NamedFunction` / `NamedFunctionContext` / `NamedFunctionTimeout` (and package-level wrappers). The wrapper invokes a caller-specified `entrypoint` function name instead of hardcoding `plugin`. `Function*` is a thin alias around `NamedFunction*` with `entrypoint = "plugin"`.
* Added `Engine.SeededFunction` / `SeededFunctionTimeout` (and package-level wrappers). Runs a JS source with a caller-supplied `bindings map[string]any` exposed as ambient lexical bindings (via IIFE destructuring), then invokes a named entry function with no arguments. Pre-flight checks reject invalid identifiers and any collision between the `entrypoint` and a binding name — that pre-flight is what protects callers from the JS scoping conflict between, e.g., `function data()` and a `data` lexical binding. Seeded calls borrow from the same runtime pool as the other entry points; callers wanting pool isolation between workloads should construct distinct `Engine`s.
* Changed `Engine.Function` / `FunctionContext` / `FunctionTimeout` (and their package-level wrappers) to drop the leading `name string` return value. They now return `(func(...any) (any, error), error)` (and the context/timeout shape variants). The compiled wrapper still requires the source to expose a top-level `plugin` callable; the JS-side check throws if it isn't there.
* Changed `plugins.LoadPlugin` to now derive the plugin's public name from the file's basename (sans `.js` extension) rather than from a `const plugin = name` selector inside the file. Plugin filenames must match `^[A-Za-z_][A-Za-z0-9_]*$` (Go identifier shape, required by `text/template` for FuncMap keys). Reserved plugin names are `get` (the value-DSL primitive) and `plugin` (the JS entry-function name).
* Changed the plugin file format to require a `function plugin(input) { ... }` (a top-level `function` declaration named `plugin`, or any expression that binds the identifier `plugin` to a function — e.g. `const plugin = (input) => ...`). The wrapper enforces `typeof plugin === "function"` at runtime.
* Changed `opmldata.UnmarshalTemplateData` and `FromOPML` no longer synthesize a top-level `namespace` key from `Head.Title`. The namespace under which OPML data is installed is now determined entirely by the loader's filename rules, not by anything inside the document.
* Removed `scripting.PluginSelector` and the entire `scripting/metadata.go` AST inspector. Plugin files no longer need a `const plugin = name` selector — their public name comes from the caller (typically the filename).

## v0.24.0 (2026-05-04)

Major refactor that splits TemplateData and the markdown engine into dedicated, dependency-isolated subpackages so callers can depend on a smaller surface and the value DSL can use markdown rendering without pulling the whole template engine. The value DSL also gains an `html/template`-backed entry point so text-node content can be rendered to `[]*html.Node`.

* Added `hypertemplates-go/data` — owns `TemplateData`, `TemplateDataSet`, dot-notation accessors, and `UnmarshalJSON` / `UnmarshalYAML`. stdlib + yaml.v3 only; zero internal deps.
* Added `hypertemplates-go/markdown` — owns `HyperMark`, `HyperMarkConfig`, `DefaultHyperMarkConfig`, `New`, `Parse`, and `UnmarshalTemplateData`. Owns the goldmark/chroma/emoji/meta dependency tree and the goldmark-meta yaml.v2 front-matter key-stringify fix-up.
* Added `hypertemplates-go/htmldata` — `UnmarshalTemplateData([]byte)`, `ExtractTemplateData(*html.Node)`, and `ExtractTemplateDataNodes([]*html.Node)` for extracting title, description, and timestamp metadata from HTML documents.
* Added `hypertemplates-go/opmldata` — `UnmarshalTemplateData([]byte)` and `FromOPML(*opml.OPML)` bridge between hyperfeed-go's OPML parser and TemplateData.
* Added `hypertemplates-go/parse` — `ReadFile(path, opts...)` and `Unmarshal(b, ext, opts...)` extension dispatcher across data, markdown, htmldata, and opmldata.
* Added `TemplateData.Get(key) any` with dot-notation key resolution and 1-indexed slice access.
* Added `TemplateData.Set(key, value) error` — now dot-walking; replaces both the old top-level `Set` and `SetKeyValue` / `SetPathValue`.
* Added `TemplateData.Has(key) bool` — replaces `Exists`. Returns true for present non-empty values.
* Added `TemplateData.Delete(key)` with dot-walking support.
* Added `TemplateData.Keys(key) []string` and `TemplateData.Len(key) int` for explicit introspection.
* Added function-style accessors as the canonical way to call computed accessors from a key: `page.tags.length()`, `page.tags.count()`, `page.metadata.keys()`. Built-in only — no plugin extensibility.
* Added `TemplateData.Without(keys ...string) TemplateData`, replacing `Clean(reserved []string)`.
* Added `TemplateData.MergeAppend(other) TemplateData` to make slice-append merging an explicit method instead of a boolean flag.
* Added `TemplateData.StringifyKeys() TemplateData` that returns a copy (renamed from `StringifyMapKeys`, no longer mutates).
* Added `value.RenderNodes(source, data, parent, funcs...) ([]*html.Node, error)` — new entry point for text-node content. Returns parsed `*html.Node` slices so funcmap entries returning `html/template.HTML` are spliced into the output as real elements rather than escaped text. Backed by `html/template`, which auto-escapes user data while passing `template.HTML` through raw. Plain text inputs come back as a single `TextNode`.
* Added `htmlContent(parent, value) ([]*html.Node, error)` and `markdownContent(parent, config, value) ([]*html.Node, error)` in the top-level package — primary helpers shared by `Template.RenderContent`, `Template.RenderParams`, and the new value-funcmap wrappers. Single source of truth for fragment parsing.
* Added `htmlValueFunc(parent)` and `markdownValueFunc(parent, config)` — funcmap-compatible wrappers that `Template.RenderValues` injects per text-node call site, so `${ page.body | html }` and `${ page.body | markdown }` work in value expressions.
* Fixed `Template.ParseParameters` to skip empty segments after splitting on `;`. Previously, an empty segment (e.g. from the unconditional `"; "` prefix in `RenderAttributes` when no `ht-attrs` attribute was present alongside an `ht-attr`) would create a `Parameter` with empty `Keys`. Combined with the new `data.Get("")` returning the entire receiver, that empty key resolved to the whole `TemplateData`, which `RenderAttributes` then dumped as attributes on the element via the `case TemplateData:` map-iteration branch. Symptom: builds bloated 70x+ (e.g. 2.7MB → 197MB on a typical site) when any element used `ht-attr` without a sibling `ht-attrs`.

* Changed `HyperMark.FrontMatter` to `map[string]any` instead of `TemplateData`. The TemplateData-flavored `Data()` method moved to `(HyperMark).TemplateData()` in the new markdown package; `ApplyMentions` is now an internal helper.
* Changed `HyperMark` construction to `markdown.New(config)` rather than `HyperMark{}.New(config)`.
* Changed (renamed) `value.Render` to `value.RenderString` to disambiguate the string-returning attribute path from the new node-returning text-node path. Update call sites accordingly.
* Changed the value rewriter to reject any pipeline command whose first token is not registered as either a built-in (`get`, `cssDefault`) or in the caller-supplied `funcs` map. This applies at both pipeline head and tail. Previously, `text/template`'s inherited builtins (`printf`, `len`, `index`, `print`, `println`, `and`, `or`, `not`, `call`, `eq`, `ne`, `lt`, `le`, `gt`, `ge`, `html`, `js`, `urlquery`) were silently callable in pipe-tail position. They now produce a `function "X" not registered` error at parse/validate time. Closes a future-proofing gap; new builtins introduced by future `text/template` or `html/template` versions are blocked by default.
* Changed head-position function-call shape (e.g. `${ printf "%s" "x" }`, `${ if page.title }`, `${ foo bar }`) to produce a `function "X" not registered` error instead of the misleading `invalid keypath` error.

* Removed the magic-suffix accessors `Get("foo.keys")` and `Get("foo.length")`. Use the function-style accessors `Get("foo.keys()")` and `Get("foo.length()")` instead. The old form silently shadowed real property names called `keys` or `length`.
* Removed `GetValue([]string)`, `GetKeys([]string)`, `GetLength([]string)`, `SetKeyValue`, `SetPathValue` — folded into `Get` / `Set` / `Keys` / `Len`.
* Removed `Lookup` (use `Get` — the docs guarantee missing-key lookups are not errors).
* Removed `Exists` (use `Has`).
* Removed `Inspect(path, format)` — `json.Marshal(d.Get(key))` covers it.
* Removed `Validate(required)` — trivially recreated as `for _, k := range required { if !d.Has(k) {...} }`.
* Removed `IsEmpty()` — use `len(d) == 0`.
* Removed `Clean(reserved []string)` — replaced by `Without(keys ...string)`.
* Removed `Merge(other, appendMode bool, verbose bool)` — split into `Merge(other)` and `MergeAppend(other)`. Verbose logging is the caller's concern.
* Removed `Namespace()`, `NamespaceData()`, `ColumnValues()` — moved to `hypertexting-go` as free functions and the `NamespacedData` wrapper type.
* Removed `OrderedTemplateData` — moved to `hypertexting-go` (the field-order list is the Page schema, which is SSG-domain).

The root `hypertemplates` package retains type aliases (`type TemplateData = data.TemplateData`, `type HyperMark = markdown.HyperMark`, etc.) and thin wrapper functions for `ParseTemplateData{File,Bytes,OPML,Markdown,HTML}` so existing import paths and call signatures continue to compile. New code should depend on the format-specific packages directly.

Method count on `TemplateData` drops from ~22 to 11. Doc comments now mirror the website's "template data object / property / key / value" vocabulary. Breaking changes have been removed without a deprecation period since the v0.x line is pre-1.0.

## v0.23.0 (2026-04-30)

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
