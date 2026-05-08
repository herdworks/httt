// Package scripting embeds a pure-Go JavaScript runtime (goja) for evaluating
// small JavaScript scripts and plugin function files against per-call Go
// arguments.
//
// Two evaluation surfaces are exposed:
//
//   - Eval/Script: inline expression evaluation against a per-call data
//     context. Top-level map keys can become local constants inside the
//     script, or a single `input` constant when the arg isn't a plain map.
//
//   - Function/Plugin: compile a JavaScript file whose default export is a
//     function (idiomatic ES module syntax: `export default function(...)`).
//     The engine pre-scans for unsupported features, transforms the source
//     through esbuild, and wraps the result in an IIFE that synthesizes
//     module/exports plus a require() stub. Plugin-level callers can supply
//     bindings that the wrapper exposes as top-level lexical locals.
package scripting

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dop251/goja"
)

// Sentinel errors returned by Eval.
var (
	// ErrClosed is returned when Eval is called on an engine that has been Close()d.
	ErrClosed = errors.New("scripting: engine closed")

	// ErrInterrupted is returned when a script is halted by the watchdog
	// (ctx cancellation, ctx deadline, or EvalTimeout elapsed).
	ErrInterrupted = errors.New("scripting: script interrupted")
)

// Options configures a new Engine. The zero value is valid.
type Options struct {
	// EvalTimeout is an optional wall-clock deadline applied to each Eval
	// call in addition to any deadline on the caller's context. Zero
	// disables the engine-level timeout (ctx still applies).
	EvalTimeout time.Duration
}

// Engine evaluates JavaScript scripts and plugin function files against
// per-call data contexts.
//
// An Engine pools a single set of hardened goja runtimes, shared across all
// evaluation entry points. Each entry point's wrapper is an IIFE whose
// locals are scoped to the call, so no per-call data leaks across runtimes
// that pass through the pool. Callers that want pool isolation between
// workloads should construct distinct Engines.
//
// An Engine is safe for concurrent use by multiple goroutines.
type Engine struct {
	opts   Options
	pool   sync.Pool // of *goja.Runtime
	cache  sync.Map  // cacheKey(string) -> *goja.Program
	closed atomic.Bool
}

// NewEngine constructs an Engine with an empty compiled-program cache and a
// lazy sync.Pool of goja runtimes.
func NewEngine(opts Options) *Engine {
	e := &Engine{opts: opts}
	e.pool.New = func() any { return newRuntime() }
	return e
}

// Close marks the engine as closed; subsequent Eval calls return ErrClosed.
// Pooled runtimes are left to the GC.
func (e *Engine) Close() error {
	e.closed.Store(true)
	return nil
}

// Eval executes src as a JavaScript script body against arg.
//
// If arg is map[string]any, its top-level keys become local constants inside
// the script. Otherwise the script receives a single local constant
// named input.
//
// ctx cancellation or its deadline triggers Runtime.Interrupt on the borrowed
// runtime, yielding ErrInterrupted.
func (e *Engine) Eval(ctx context.Context, src string, arg any) (any, error) {
	v, err := e.evalValue(ctx, src, arg)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, nil
	}
	return v.Export(), nil
}

// EvalString evaluates src and returns its value as a Go string, applying
// JavaScript's ToString coercion (the same coercion template literals use
// internally to stringify their ${…} slots). Numbers become "42", booleans
// become "true"/"false", null becomes "null", undefined becomes "undefined",
// plain objects become "[object Object]".
//
// This is the recommended entry point for template-literal-based rendering.
func (e *Engine) EvalString(ctx context.Context, src string, arg any) (string, error) {
	v, err := e.evalValue(ctx, src, arg)
	if err != nil {
		return "", err
	}
	if v == nil {
		return "", nil
	}
	return v.String(), nil
}

// EvalStringStrict behaves like EvalString but returns an error if the
// script did not return a JavaScript string. Use this when a non-string result
// would indicate a bug in the script or data shape.
func (e *Engine) EvalStringStrict(ctx context.Context, src string, arg any) (string, error) {
	v, err := e.evalValue(ctx, src, arg)
	if err != nil {
		return "", err
	}
	if v == nil {
		return "", fmt.Errorf("scripting: EvalStringStrict: expected string, got undefined/null")
	}
	exported := v.Export()
	s, ok := exported.(string)
	if !ok {
		return "", fmt.Errorf("scripting: EvalStringStrict: expected string, got %T", exported)
	}
	return s, nil
}

// Script returns a reusable evaluator that applies a fixed timeout to every
// call. sampleArg defines the binding mode and top-level contract for later
// invocations.
func (e *Engine) Script(src string, sampleArg any) (func(any) (any, error), error) {
	return e.ScriptTimeout(src, sampleArg, defaultFunctionTimeout)
}

// ScriptContext returns a reusable evaluator that accepts a caller-owned
// context on every invocation. sampleArg defines the binding mode and
// top-level contract for later invocations.
func (e *Engine) ScriptContext(src string, sampleArg any) (func(context.Context, any) (any, error), error) {
	shape := bindingShapeForArg(sampleArg)
	prog, err := e.programForShape(src, shape)
	if err != nil {
		return nil, err
	}

	return func(ctx context.Context, arg any) (any, error) {
		if err := shape.validateCallArg(arg); err != nil {
			return nil, err
		}

		v, err := e.evalProgram(ctx, prog, arg)
		if err != nil {
			return nil, err
		}
		if v == nil {
			return nil, nil
		}
		return v.Export(), nil
	}, nil
}

// ScriptTimeout returns a reusable evaluator that applies timeout to every
// call. sampleArg defines the binding mode and top-level contract for later
// invocations.
func (e *Engine) ScriptTimeout(src string, sampleArg any, timeout time.Duration) (func(any) (any, error), error) {
	if timeout <= 0 {
		return nil, fmt.Errorf("scripting: ScriptTimeout: timeout must be greater than zero")
	}

	fn, err := e.ScriptContext(src, sampleArg)
	if err != nil {
		return nil, err
	}

	return func(arg any) (any, error) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		return fn(ctx, arg)
	}, nil
}

// Function compiles a JavaScript file whose default export is a function
// into a reusable Go callable. Each invocation passes ordered Go arguments
// to JavaScript as positional arguments to the default-exported function.
// The default per-call timeout is applied.
//
// The source must be ES module-shaped (`export default function(...)`) or
// CJS-shaped (`module.exports = function(...)`). esbuild handles the
// transformation; see scripting/cache.go pluginWrapperSource for the
// extraction logic.
func (e *Engine) Function(src string) (func(...any) (any, error), error) {
	return e.FunctionTimeout(src, defaultFunctionTimeout)
}

// FunctionContext is the context-accepting variant of Function.
func (e *Engine) FunctionContext(src string) (func(context.Context, ...any) (any, error), error) {
	transformed, err := transform(src, "")
	if err != nil {
		return nil, err
	}

	prog, err := e.programForPlugin(transformed, nil)
	if err != nil {
		return nil, err
	}

	return func(ctx context.Context, args ...any) (any, error) {
		v, err := e.evalPluginProgram(ctx, prog, nil, args)
		if err != nil {
			if perr := asPluginError(err, ""); perr != nil {
				return nil, perr
			}
			return nil, err
		}
		if v == nil {
			return nil, nil
		}
		return v.Export(), nil
	}, nil
}

// FunctionTimeout is the timeout variant of Function.
func (e *Engine) FunctionTimeout(src string, timeout time.Duration) (func(...any) (any, error), error) {
	if timeout <= 0 {
		return nil, fmt.Errorf("scripting: FunctionTimeout: timeout must be greater than zero")
	}

	fn, err := e.FunctionContext(src)
	if err != nil {
		return nil, err
	}

	return func(args ...any) (any, error) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		return fn(ctx, args...)
	}, nil
}

func isValidJSIdentifier(name string) bool {
	if name == "" {
		return false
	}
	return bindingKeyIsValid(name)
}

func (e *Engine) ensureUsable(ctx context.Context) error {
	if e.closed.Load() {
		return ErrClosed
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

// evalValue is the shared core for Eval/Script: compile (or reuse), borrow a
// runtime, run, and return the raw goja.Value. Public methods wrap this with
// the appropriate unwrap strategy (Export, String, strict assertion, etc.).
func (e *Engine) evalValue(ctx context.Context, src string, arg any) (goja.Value, error) {
	if err := e.ensureUsable(ctx); err != nil {
		return nil, err
	}

	prog, err := e.programForArg(src, arg)
	if err != nil {
		return nil, err
	}

	return e.evalProgram(ctx, prog, arg)
}

func (e *Engine) evalProgram(ctx context.Context, prog *goja.Program, arg any) (goja.Value, error) {
	if err := e.ensureUsable(ctx); err != nil {
		return nil, err
	}

	vm := e.pool.Get().(*goja.Runtime)
	defer e.pool.Put(vm)

	stop := e.contextSupervisor(ctx, vm)
	defer stop()

	fnVal, err := vm.RunProgram(prog)
	if err != nil {
		return nil, wrapRunError(err, ctx)
	}
	fn, ok := goja.AssertFunction(fnVal)
	if !ok {
		return nil, fmt.Errorf("scripting: compiled wrapper is not callable")
	}

	argVal := vm.ToValue(jsPlainValue(arg))
	result, err := fn(goja.Undefined(), argVal)
	if err != nil {
		return nil, wrapRunError(err, ctx)
	}
	return result, nil
}

// evalPluginProgram runs a compiled plugin wrapper. The wrapper IIFE takes
// (__bindings, __args); bindings is forwarded as a JS object (or null when
// nil), and args is forwarded as a JS array. The wrapper internally
// destructures bindings (if its keys list is non-empty) and applies args
// to the default-exported function.
func (e *Engine) evalPluginProgram(ctx context.Context, prog *goja.Program, bindings map[string]any, args []any) (goja.Value, error) {
	if err := e.ensureUsable(ctx); err != nil {
		return nil, err
	}

	vm := e.pool.Get().(*goja.Runtime)
	defer e.pool.Put(vm)

	stop := e.contextSupervisor(ctx, vm)
	defer stop()

	fnVal, err := vm.RunProgram(prog)
	if err != nil {
		return nil, wrapRunError(err, ctx)
	}
	fn, ok := goja.AssertFunction(fnVal)
	if !ok {
		return nil, fmt.Errorf("scripting: compiled plugin wrapper is not callable")
	}

	var bindingsVal goja.Value = goja.Null()
	if bindings != nil {
		bindingsVal = vm.ToValue(jsPlainValue(bindings))
	}

	argsForJS := make([]any, 0, len(args))
	for _, a := range args {
		argsForJS = append(argsForJS, jsPlainValue(a))
	}
	argsVal := vm.ToValue(argsForJS)

	result, err := fn(goja.Undefined(), bindingsVal, argsVal)
	if err != nil {
		return nil, wrapRunError(err, ctx)
	}
	return result, nil
}

// contextSupervisor starts a goroutine that interrupts vm if ctx is cancelled or
// the EvalTimeout elapses. The returned stop() must be called when the
// script has finished executing; it tears the watchdog down and clears any
// pending interrupt so the runtime is clean for the next borrower.
func (e *Engine) contextSupervisor(ctx context.Context, vm *goja.Runtime) func() {
	ctxDone := ctx.Done()
	if ctxDone == nil && e.opts.EvalTimeout <= 0 {
		return func() {}
	}

	done := make(chan struct{})
	finished := make(chan struct{})
	var timer *time.Timer
	var timerC <-chan time.Time
	if e.opts.EvalTimeout > 0 {
		timer = time.NewTimer(e.opts.EvalTimeout)
		timerC = timer.C
	}

	go func() {
		defer close(finished)

		select {
		case <-ctxDone:
			vm.Interrupt(ctx.Err())
		case <-timerC:
			vm.Interrupt(context.DeadlineExceeded)
		case <-done:
		}
	}()

	return func() {
		close(done)
		if timer != nil {
			timer.Stop()
		}
		<-finished
		vm.ClearInterrupt()
	}
}

// wrapRunError normalizes a goja error into one of the package's sentinel
// errors where appropriate, preserving the underlying detail via errors.Join.
func wrapRunError(err error, ctx context.Context) error {
	if err == nil {
		return nil
	}
	var interrupted *goja.InterruptedError
	if errors.As(err, &interrupted) {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return errors.Join(ErrInterrupted, ctxErr, err)
		}
		return errors.Join(ErrInterrupted, err)
	}
	return err
}
