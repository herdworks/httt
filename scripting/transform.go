package scripting

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
)

// pluginGlobalName is the binding esbuild assigns the module's exports
// object to in IIFE-formatted output. The wrapper reads __pluginMod.default
// to extract the plugin function. Using a `__`-prefixed, project-specific
// name avoids collisions with esbuild's own `__defProp`/`__export`/etc.
// helpers and with anything a plugin author is realistically likely to
// declare.
const pluginGlobalName = "__pluginMod"

// transform converts an ES module-shaped plugin source into a self-contained
// IIFE that assigns the module's exports to __pluginMod. The pipeline is:
//
//  1. Pre-scan for unsupported syntax (currently `import` declarations and
//     `export ... from` re-exports). If found, return a *PluginError so
//     callers can fail fast at load time rather than at evaluation time.
//
//  2. esbuild.Transform with FormatIIFE / Target=ES2020 / GlobalName set.
//     esbuild handles `export default`, named exports, hoisting, classes,
//     async functions, etc., and emits `var __pluginMod = (() => {…})();`
//     The plugin wrapper then reads `__pluginMod.default`.
//
// Authors must use ES module syntax (`export default <function>`). CJS-style
// `module.exports = X` is rejected by esbuild in IIFE format.
func transform(src, path string) (string, error) {
	if err := preScanForUnsupported(src, path); err != nil {
		return "", err
	}

	result := api.Transform(src, api.TransformOptions{
		Loader:     api.LoaderJS,
		Format:     api.FormatIIFE,
		GlobalName: pluginGlobalName,
		Target:     api.ES2020,
		Sourcefile: path,
	})
	if len(result.Errors) > 0 {
		return "", esbuildError(result.Errors, path)
	}

	return string(result.Code), nil
}

// importDeclRegexp matches a top-level `import` declaration. Anchored at a
// line start (with optional leading whitespace) so common cases like
// `// import x` or `import.meta` mid-line do not trigger.
var importDeclRegexp = regexp.MustCompile(`(?m)^\s*import\s`)

// exportFromRegexp matches `export ... from '...'` re-export forms. Covers
// `export { x } from`, `export * from`, and `export * as ns from`. The
// non-`from` `export` cases (e.g. `export default ...`, `export const x`) are
// allowed and handled by esbuild.
var exportFromRegexp = regexp.MustCompile(`(?m)^\s*export\s.+\sfrom\s`)

// preScanForUnsupported rejects plugin sources that use language features
// the HyperTemplates plugin runtime does not (yet) support. This is a
// best-effort line-anchored regex scan; pathological inputs (e.g. a block
// comment or template literal whose content begins with `import ` at a
// line start) may produce false positives. Such cases are rare in plugin
// files and easily worked around by reformatting the source.
func preScanForUnsupported(src, path string) error {
	if importDeclRegexp.MatchString(src) {
		return &PluginError{Feature: "import", Path: path}
	}
	if exportFromRegexp.MatchString(src) {
		return &PluginError{Feature: "import", Path: path}
	}
	return nil
}

// esbuildError converts an esbuild error slice into a single Go error,
// preserving the first error's message and location for diagnostic value
// without leaking arbitrarily-long source excerpts.
func esbuildError(errs []api.Message, path string) error {
	if len(errs) == 0 {
		return nil
	}
	first := errs[0]
	loc := ""
	if first.Location != nil {
		loc = fmt.Sprintf(" (line %d:%d)", first.Location.Line, first.Location.Column)
	}
	return fmt.Errorf("scripting: esbuild transform %q failed%s: %s", truncate(path, 120), loc, first.Text)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// pluginCacheKey returns the cache key used for compiled plugin programs.
// Distinct from script/eval cache keys (which use bindingShape) and avoids
// any collision with the bindingShape key namespace.
func pluginCacheKey(transformedSrc string, sortedKeys []string) string {
	var b strings.Builder
	b.Grow(len(transformedSrc) + 32)
	b.WriteString("plugin\x00")
	for _, k := range sortedKeys {
		b.WriteString(k)
		b.WriteByte(0)
	}
	b.WriteByte(0)
	b.WriteString(transformedSrc)
	return b.String()
}
