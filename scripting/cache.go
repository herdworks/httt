package scripting

import (
	"fmt"

	"github.com/dop251/goja"
)

// programForArg returns the compiled *goja.Program for (expr, arg-shape),
// compiling and caching it on a miss.
func (e *Engine) programForArg(expr string, arg any) (*goja.Program, error) {
	return e.programForShape(expr, bindingShapeForArg(arg))
}

// programForShape returns the compiled *goja.Program for (expr, binding-shape),
// compiling and caching it on a miss.
func (e *Engine) programForShape(expr string, shape bindingShape) (*goja.Program, error) {
	key := shape.cacheKey(expr)

	if cached, ok := e.cache.Load(key); ok {
		return cached.(*goja.Program), nil
	}

	if err := shape.validateLocalBindings(); err != nil {
		return nil, err
	}

	wrapped := shape.wrapperSource(expr)
	prog, err := goja.Compile("cache.go", wrapped, true)
	if err != nil {
		return nil, compileError(expr, err)
	}

	actual, _ := e.cache.LoadOrStore(key, prog)
	return actual.(*goja.Program), nil
}

// compileError wraps a goja compile error with a short source excerpt so
// callers can identify which expression failed without leaking an
// unbounded amount of user-supplied text.
func compileError(src string, err error) error {
	const max = 80
	excerpt := src
	if len(excerpt) > max {
		excerpt = excerpt[:max] + "…"
	}
	return fmt.Errorf("scripting: compile %q: %w", excerpt, err)
}
