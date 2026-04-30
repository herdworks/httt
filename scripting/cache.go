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
// compiling and caching it on a miss.
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

func (e *Engine) programForPluginFunction(src string) (*goja.Program, error) {
	const cachePrefix = "plugin-function"
	key := cachePrefix + "\x00" + src

	if cached, ok := e.cache.Load(key); ok {
		return cached.(*goja.Program), nil
	}

	wrapped := pluginFunctionWrapperSource(src)
	prog, err := goja.Compile("cache.go", wrapped, true)
	if err != nil {
		return nil, compileError(src, err)
	}

	actual, _ := e.cache.LoadOrStore(key, prog)
	return actual.(*goja.Program), nil
}

func pluginFunctionWrapperSource(src string) string {
	var b strings.Builder
	b.Grow(len(src) + 128)
	b.WriteString(`(function(__args){ "use strict"; `)
	b.WriteString(src)
	b.WriteString("\n")
	b.WriteString(`; if (typeof plugin !== "function") { throw new TypeError("plugin selector is not callable"); } return plugin(...__args); })`)
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
