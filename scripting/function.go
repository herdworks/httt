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

// Function returns a reusable evaluator backed by the package-default engine.
// Each invocation uses a fixed 250ms timeout.
func Function(expr string, sampleArg any) (func(any) (any, error), error) {
	return shared().Function(expr, sampleArg)
}

// FunctionContext returns a reusable evaluator backed by the package-default
// engine. Callers own the context for every invocation.
func FunctionContext(expr string, sampleArg any) (func(context.Context, any) (any, error), error) {
	return shared().FunctionContext(expr, sampleArg)
}

// FunctionTimeout returns a reusable evaluator backed by the package-default
// engine. Each invocation uses timeout in addition to any engine-level limits.
func FunctionTimeout(expr string, sampleArg any, timeout time.Duration) (func(any) (any, error), error) {
	return shared().FunctionTimeout(expr, sampleArg, timeout)
}
