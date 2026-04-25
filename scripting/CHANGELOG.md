# Changelog

All notable changes to this project will be documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/).
This project adheres to [Semantic Versioning](https://semver.org/).

## Unreleased

### Added

* Added the initial `scripting` package for evaluating JavaScript function bodies with pooled `goja` runtimes.
* Added concurrency-safe caching, cancellation, and timeout behavior for script evaluation.
* Added README documentation with installation instructions and usage examples.
* Added compiled-function APIs on `Engine` and package-level `Function`, `FunctionContext`, and `FunctionTimeout` helpers backed by a lazy default engine.
* Added `PluginName` for parsing leading `ht:plugin` JavaScript metadata comments.

### Changed

* Changed script binding to expose either top-level `map[string]any` keys or a single `input` constant for every non-map argument.
* Changed scripting APIs to evaluate strict-mode function bodies that use explicit `return` values.
* Changed `Eval`, `EvalString`, and `EvalStringStrict` to accept `any` arguments instead of requiring `map[string]any`.
* Changed pooled runtime initialization to freeze `globalThis`, `Object`, `Object.prototype`, `Array`, `Array.prototype`, `Function`, and `Function.prototype` so broad cross-evaluation monkey-patching fails fast.
* Changed watchdog teardown to wait for timeout and cancellation interrupt handlers to finish before returning a runtime to the pool.
* Changed map-binding errors to reject invalid strict-mode JavaScript top-level binding names with a clear error before wrapper compilation.
