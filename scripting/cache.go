package scripting

import (
	"fmt"
	"strings"

	"github.com/dop251/goja"
)

// programForArg returns the compiled *goja.Program for (src, arg-shape),
// compiling and caching it on a miss.
func (e *Engine) programForArg(src string, arg any) (*goja.Program, error) {
	return e.programForShape(src, bindingShapeForArg(arg))
}

// programForShape returns the compiled *goja.Program for (src, binding-shape),
// compiling and caching it on a miss. Used by Eval/Script — i.e. inline
// expression evaluation against a per-call data context.
func (e *Engine) programForShape(src string, shape bindingShape) (*goja.Program, error) {
	key := shape.cacheKey(src)

	if cached, ok := e.cache.Load(key); ok {
		return cached.(*goja.Program), nil
	}

	if err := shape.validateLocalBindings(); err != nil {
		return nil, err
	}

	wrapped := shape.wrapperSource(src)
	prog, err := goja.Compile("cache.go", wrapped, true)
	if err != nil {
		return nil, compileError(src, err)
	}

	actual, _ := e.cache.LoadOrStore(key, prog)
	return actual.(*goja.Program), nil
}

// programForPlugin returns the compiled *goja.Program for an esbuild-
// transformed plugin source wrapped in the plugin IIFE, parameterized by
// the sorted bindings key list. Plugin sources are expected to be CJS-
// shaped (the output of esbuild Transform with FormatCommonJS).
func (e *Engine) programForPlugin(transformedSrc string, sortedKeys []string) (*goja.Program, error) {
	key := pluginCacheKey(transformedSrc, sortedKeys)

	if cached, ok := e.cache.Load(key); ok {
		return cached.(*goja.Program), nil
	}

	wrapped := pluginWrapperSource(transformedSrc, sortedKeys)
	prog, err := goja.Compile("cache.go", wrapped, true)
	if err != nil {
		return nil, compileError(transformedSrc, err)
	}

	actual, _ := e.cache.LoadOrStore(key, prog)
	return actual.(*goja.Program), nil
}

// pluginWrapperSource produces the outer IIFE that wraps an esbuild-
// transformed plugin source. The wrapper takes two arguments —
// `__bindings` and `__args` — optionally destructures bindings into
// top-level locals, defines a `require` stub, then runs the esbuild IIFE
// (which assigns the module's exports to the global named in
// pluginGlobalName, e.g. __pluginMod). After the inner IIFE returns, the
// wrapper extracts __pluginMod.default, validates it's callable, and
// invokes it with the supplied args.
//
// The require stub throws a JS Error tagged with name === "PluginError"
// and pluginErrorFeature === "require"; asPluginError converts those back
// to a *PluginError on the Go side so callers can use errors.As.
//
// To re-enable CJS-style authoring (`module.exports = function(){…}`) in
// the future:
//  1. transform.go: switch `Format: api.FormatIIFE` back to
//     `api.FormatCommonJS` and remove `GlobalName: pluginGlobalName`.
//  2. Below: re-introduce `const module = { exports: {} }; const exports
//     = module.exports;` before the user source, and replace the
//     extraction line with the dual-form fallback
//     `(module.exports && module.exports.default !== undefined) ?
//     module.exports.default : module.exports`. The require stub
//     and bindings prelude stay as-is.
func pluginWrapperSource(transformedSrc string, sortedKeys []string) string {
	var b strings.Builder
	b.Grow(len(transformedSrc) + 512)

	b.WriteString(`(function(__bindings, __args) { "use strict"; `)

	if len(sortedKeys) > 0 {
		b.WriteString(`const {`)
		b.WriteString(strings.Join(sortedKeys, ","))
		b.WriteString(`} = __bindings; `)
	}

	b.WriteString(`const require = function(__spec) { `)
	b.WriteString(`const __err = new Error("PluginError: require is not supported in plugins yet"); `)
	b.WriteString(`__err.name = "PluginError"; `)
	b.WriteString(`__err.pluginErrorFeature = "require"; `)
	b.WriteString(`throw __err; `)
	b.WriteString(`}; `)

	b.WriteString("\n")
	b.WriteString(transformedSrc)
	b.WriteString("\n")

	b.WriteString(`const __plugin = (typeof `)
	b.WriteString(pluginGlobalName)
	b.WriteString(` !== "undefined") ? `)
	b.WriteString(pluginGlobalName)
	b.WriteString(`.default : undefined; `)
	b.WriteString(`if (typeof __plugin !== "function") { throw new TypeError("plugin default export is not a function"); } `)
	b.WriteString(`return __plugin.apply(undefined, __args || []); })`)

	return b.String()
}

// compileError wraps a goja compile error with a short source excerpt so
// callers can identify which script failed without leaking an
// unbounded amount of user-supplied text.
func compileError(src string, err error) error {
	const max = 80
	excerpt := src
	if len(excerpt) > max {
		excerpt = excerpt[:max] + "…"
	}
	return fmt.Errorf("scripting: compile %q: %w", excerpt, err)
}
