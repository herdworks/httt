package scripting

import (
	"context"
	"sync"
	"time"
)

const defaultFunctionTimeout = 250 * time.Millisecond

var sharedEngine struct {
	once   sync.Once
	engine *Engine
}

func shared() *Engine {
	sharedEngine.once.Do(func() {
		sharedEngine.engine = NewEngine(Options{})
	})

	return sharedEngine.engine
}

// Script returns a reusable evaluator backed by the package-default engine.
// Each invocation uses a fixed 250ms timeout.
func Script(src string, sampleArg any) (func(any) (any, error), error) {
	return shared().Script(src, sampleArg)
}

// ScriptContext returns a reusable evaluator backed by the package-default
// engine. Callers own the context for every invocation.
func ScriptContext(src string, sampleArg any) (func(context.Context, any) (any, error), error) {
	return shared().ScriptContext(src, sampleArg)
}

// ScriptTimeout returns a reusable evaluator backed by the package-default
// engine. Each invocation uses timeout in addition to any engine-level limits.
func ScriptTimeout(src string, sampleArg any, timeout time.Duration) (func(any) (any, error), error) {
	return shared().ScriptTimeout(src, sampleArg, timeout)
}

// Function compiles a JavaScript plugin file using the package-default engine.
// The source must define a top-level `function plugin(...)`. Each invocation
// uses a fixed 250ms timeout.
func Function(src string) (func(...any) (any, error), error) {
	return shared().Function(src)
}

// FunctionContext compiles a JavaScript plugin file using the package-default
// engine. Callers own the context for every invocation.
func FunctionContext(src string) (func(context.Context, ...any) (any, error), error) {
	return shared().FunctionContext(src)
}

// FunctionTimeout compiles a JavaScript plugin file using the package-default
// engine. Each invocation uses timeout in addition to any engine-level limits.
func FunctionTimeout(src string, timeout time.Duration) (func(...any) (any, error), error) {
	return shared().FunctionTimeout(src, timeout)
}

// NamedFunction compiles a JavaScript file that defines a top-level function
// named `entrypoint` using the package-default engine.
func NamedFunction(src, entrypoint string) (func(...any) (any, error), error) {
	return shared().NamedFunction(src, entrypoint)
}

// NamedFunctionContext is the context-accepting variant of NamedFunction.
func NamedFunctionContext(src, entrypoint string) (func(context.Context, ...any) (any, error), error) {
	return shared().NamedFunctionContext(src, entrypoint)
}

// NamedFunctionTimeout is the timeout variant of NamedFunction.
func NamedFunctionTimeout(src, entrypoint string, timeout time.Duration) (func(...any) (any, error), error) {
	return shared().NamedFunctionTimeout(src, entrypoint, timeout)
}

// SeededFunction evaluates src with the given bindings using the
// package-default engine. See Engine.SeededFunction for full semantics.
func SeededFunction(src, entrypoint string, bindings map[string]any) (any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultFunctionTimeout)
	defer cancel()
	return shared().SeededFunction(ctx, src, entrypoint, bindings)
}

// SeededFunctionContext is the context-accepting variant of SeededFunction.
func SeededFunctionContext(ctx context.Context, src, entrypoint string, bindings map[string]any) (any, error) {
	return shared().SeededFunction(ctx, src, entrypoint, bindings)
}

// SeededFunctionTimeout is the timeout variant of SeededFunction.
func SeededFunctionTimeout(src, entrypoint string, bindings map[string]any, timeout time.Duration) (any, error) {
	return shared().SeededFunctionTimeout(src, entrypoint, bindings, timeout)
}
