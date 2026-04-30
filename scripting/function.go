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
// Each invocation uses a fixed 250ms timeout.
func Function(src string) (string, func(...any) (any, error), error) {
	return shared().Function(src)
}

// FunctionContext compiles a JavaScript plugin file using the package-default
// engine. Callers own the context for every invocation.
func FunctionContext(src string) (string, func(context.Context, ...any) (any, error), error) {
	return shared().FunctionContext(src)
}

// FunctionTimeout compiles a JavaScript plugin file using the package-default
// engine. Each invocation uses timeout in addition to any engine-level limits.
func FunctionTimeout(src string, timeout time.Duration) (string, func(...any) (any, error), error) {
	return shared().FunctionTimeout(src, timeout)
}
