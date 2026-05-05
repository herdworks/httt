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

func (e *Engine) programForNamedFunction(src, entrypoint string) (*goja.Program, error) {
	const cachePrefix = "named-function"
	key := cachePrefix + "\x00" + entrypoint + "\x00" + src

	if cached, ok := e.cache.Load(key); ok {
		return cached.(*goja.Program), nil
	}

	wrapped := namedFunctionWrapperSource(src, entrypoint)
	prog, err := goja.Compile("cache.go", wrapped, true)
	if err != nil {
		return nil, compileError(src, err)
	}

	actual, _ := e.cache.LoadOrStore(key, prog)
	return actual.(*goja.Program), nil
}

func namedFunctionWrapperSource(src, entrypoint string) string {
	var b strings.Builder
	b.Grow(len(src) + 160)
	b.WriteString(`(function(__args){ "use strict"; `)
	b.WriteString(src)
	b.WriteString("\n")
	b.WriteString(`; if (typeof `)
	b.WriteString(entrypoint)
	b.WriteString(` !== "function") { throw new TypeError("`)
	b.WriteString(entrypoint)
	b.WriteString(` is not a function"); } return `)
	b.WriteString(entrypoint)
	b.WriteString(`(...__args); })`)
	return b.String()
}

func (e *Engine) programForSeededFunction(src, entrypoint string, sortedKeys []string) (*goja.Program, error) {
	const cachePrefix = "seeded"
	key := cachePrefix + "\x00" + entrypoint + "\x00" + strings.Join(sortedKeys, ",") + "\x00" + src

	if cached, ok := e.cache.Load(key); ok {
		return cached.(*goja.Program), nil
	}

	wrapped := seededWrapperSource(src, entrypoint, sortedKeys)
	prog, err := goja.Compile("cache.go", wrapped, true)
	if err != nil {
		return nil, compileError(src, err)
	}

	actual, _ := e.cache.LoadOrStore(key, prog)
	return actual.(*goja.Program), nil
}

func seededWrapperSource(src, entrypoint string, sortedKeys []string) string {
	var b strings.Builder
	b.Grow(len(src) + 192)
	b.WriteString(`(function(__bindings){ "use strict"; `)
	if len(sortedKeys) > 0 {
		b.WriteString(`const {`)
		b.WriteString(strings.Join(sortedKeys, ","))
		b.WriteString(`} = __bindings; `)
	}
	b.WriteString(src)
	b.WriteString("\n")
	b.WriteString(`; if (typeof `)
	b.WriteString(entrypoint)
	b.WriteString(` !== "function") { throw new TypeError("`)
	b.WriteString(entrypoint)
	b.WriteString(` is not a function"); } return `)
	b.WriteString(entrypoint)
	b.WriteString(`(); })`)
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
