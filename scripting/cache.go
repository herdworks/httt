package scripting

import (
	"fmt"

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
