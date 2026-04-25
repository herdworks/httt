// Package scripting embeds a pure-Go JavaScript runtime (goja) for evaluating
// template-literal-style expressions against per-call Go arguments.
//
// The engine compiles expressions into reusable bytecode wrappers that either
// expose top-level map keys as local constants or bind a single local
// `input` constant, so per-call values never leak across calls sharing the
// same pooled runtime.
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

// Engine evaluates JavaScript expressions against per-call data contexts.
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

// Eval executes expr as a JavaScript expression against arg.
//
// If arg is map[string]any, its top-level keys become local constants inside
// the expression. Otherwise the expression receives a single local constant
// named input.
//
// ctx cancellation or its deadline triggers Runtime.Interrupt on the borrowed
// runtime, yielding ErrInterrupted.
func (e *Engine) Eval(ctx context.Context, expr string, arg any) (any, error) {
	v, err := e.evalValue(ctx, expr, arg)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, nil
	}
	return v.Export(), nil
}

// EvalString evaluates expr and returns its value as a Go string, applying
// JavaScript's ToString coercion (the same coercion template literals use
// internally to stringify their ${…} slots). Numbers become "42", booleans
// become "true"/"false", null becomes "null", undefined becomes "undefined",
// plain objects become "[object Object]".
//
// This is the recommended entry point for template-literal-based rendering.
func (e *Engine) EvalString(ctx context.Context, expr string, arg any) (string, error) {
	v, err := e.evalValue(ctx, expr, arg)
	if err != nil {
		return "", err
	}
	if v == nil {
		return "", nil
	}
	return v.String(), nil
}

// EvalStringStrict behaves like EvalString but returns an error if the
// expression did not evaluate to a JavaScript string. Use this when a
// non-string result would indicate a bug in the expression or data shape.
func (e *Engine) EvalStringStrict(ctx context.Context, expr string, arg any) (string, error) {
	v, err := e.evalValue(ctx, expr, arg)
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

// Function returns a reusable evaluator that applies a fixed timeout to every
// call. sampleArg defines the binding mode and top-level contract for later
// invocations.
func (e *Engine) Function(expr string, sampleArg any) (func(any) (any, error), error) {
	return e.FunctionTimeout(expr, sampleArg, defaultFunctionTimeout)
}

// FunctionContext returns a reusable evaluator that accepts a caller-owned
// context on every invocation. sampleArg defines the binding mode and
// top-level contract for later invocations.
func (e *Engine) FunctionContext(expr string, sampleArg any) (func(context.Context, any) (any, error), error) {
	shape := bindingShapeForArg(sampleArg)
	prog, err := e.programForShape(expr, shape)
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

// FunctionTimeout returns a reusable evaluator that applies timeout to every
// call. sampleArg defines the binding mode and top-level contract for later
// invocations.
func (e *Engine) FunctionTimeout(expr string, sampleArg any, timeout time.Duration) (func(any) (any, error), error) {
	if timeout <= 0 {
		return nil, fmt.Errorf("scripting: FunctionTimeout: timeout must be greater than zero")
	}

	fn, err := e.FunctionContext(expr, sampleArg)
	if err != nil {
		return nil, err
	}

	return func(arg any) (any, error) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		return fn(ctx, arg)
	}, nil
}

// -----------------------------------------------------------------------------
// Future convenience wrappers — commented stubs for reference.
//
// The pattern established by EvalString/EvalStringStrict generalizes to other
// JS primitive and compound types. Each method is a few lines delegating to
// evalValue and differs only in how it unwraps the returned goja.Value.
//
// These are left commented out because each one involves a coercion-vs-strict
// design decision that is easier to make with a concrete caller in hand. The
// sketches below are starting points, not commitments: uncomment, revisit the
// semantics against the use case, add tests, and ship.
//
// Uncommenting EvalNumber will also require importing "math" for math.NaN().
// -----------------------------------------------------------------------------

// EvalNumber evaluates src and returns its value as a float64, applying
// JavaScript's ToNumber coercion. Strings of digits become their numeric
// value, true/false become 1/0, null becomes 0, undefined becomes NaN,
// objects become NaN (unless they define valueOf() returning a number).
//
// func (e *Engine) EvalNumber(ctx context.Context, expr string, arg any) (float64, error) {
// 	v, err := e.evalValue(ctx, expr, arg)
// 	if err != nil {
// 		return 0, err
// 	}
// 	if v == nil {
// 		return math.NaN(), nil
// 	}
// 	return v.ToFloat(), nil
// }

// EvalNumberStrict behaves like EvalNumber but returns an error if the
// expression did not evaluate to a JavaScript number. JS numbers export
// to either int64 (when the value is an integer that fits) or float64,
// so both are accepted here and normalized to float64.
//
// func (e *Engine) EvalNumberStrict(ctx context.Context, expr string, arg any) (float64, error) {
// 	v, err := e.evalValue(ctx, expr, arg)
// 	if err != nil {
// 		return 0, err
// 	}
// 	if v == nil {
// 		return 0, fmt.Errorf("scripting: EvalNumberStrict: expected number, got undefined/null")
// 	}
// 	switch n := v.Export().(type) {
// 	case int64:
// 		return float64(n), nil
// 	case float64:
// 		return n, nil
// 	default:
// 		return 0, fmt.Errorf("scripting: EvalNumberStrict: expected number, got %T", n)
// 	}
// }

// EvalBool evaluates src and returns its value as a bool, applying
// JavaScript's ToBoolean coercion. Falsy values: 0, "", null, undefined,
// NaN. Everything else is truthy — including empty arrays and empty objects,
// which is a JS quirk worth warning callers about.
//
// func (e *Engine) EvalBool(ctx context.Context, expr string, arg any) (bool, error) {
// 	v, err := e.evalValue(ctx, expr, arg)
// 	if err != nil {
// 		return false, err
// 	}
// 	if v == nil {
// 		return false, nil
// 	}
// 	return v.ToBoolean(), nil
// }

// EvalBoolStrict behaves like EvalBool but returns an error if the
// expression did not evaluate to a JavaScript boolean.
//
// func (e *Engine) EvalBoolStrict(ctx context.Context, expr string, arg any) (bool, error) {
// 	v, err := e.evalValue(ctx, expr, arg)
// 	if err != nil {
// 		return false, err
// 	}
// 	if v == nil {
// 		return false, fmt.Errorf("scripting: EvalBoolStrict: expected boolean, got undefined/null")
// 	}
// 	b, ok := v.Export().(bool)
// 	if !ok {
// 		return false, fmt.Errorf("scripting: EvalBoolStrict: expected boolean, got %T", v.Export())
// 	}
// 	return b, nil
// }

// EvalObject evaluates src and returns its value as a map[string]any.
// Lenient: null and undefined yield a nil map with no error (Go callers
// can range over a nil map safely). Plain JS objects export directly.
// Arrays — which goja exports as []any — will fail the type assertion;
// callers that want arrays should use Eval and type-assert themselves.
//
// func (e *Engine) EvalObject(ctx context.Context, expr string, arg any) (map[string]any, error) {
// 	v, err := e.evalValue(ctx, expr, arg)
// 	if err != nil {
// 		return nil, err
// 	}
// 	if v == nil {
// 		return nil, nil
// 	}
// 	exported := v.Export()
// 	if exported == nil {
// 		return nil, nil
// 	}
// 	m, ok := exported.(map[string]any)
// 	if !ok {
// 		return nil, fmt.Errorf("scripting: EvalObject: expected object, got %T", exported)
// 	}
// 	return m, nil
// }

// EvalObjectStrict behaves like EvalObject but rejects null/undefined and
// anything else that isn't a plain JS object.
//
// func (e *Engine) EvalObjectStrict(ctx context.Context, expr string, arg any) (map[string]any, error) {
// 	v, err := e.evalValue(ctx, expr, arg)
// 	if err != nil {
// 		return nil, err
// 	}
// 	if v == nil {
// 		return nil, fmt.Errorf("scripting: EvalObjectStrict: expected object, got undefined/null")
// 	}
// 	m, ok := v.Export().(map[string]any)
// 	if !ok {
// 		return nil, fmt.Errorf("scripting: EvalObjectStrict: expected object, got %T", v.Export())
// 	}
// 	return m, nil
// }

// -----------------------------------------------------------------------------

func (e *Engine) ensureUsable(ctx context.Context) error {
	if e.closed.Load() {
		return ErrClosed
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

// evalValue is the shared core: compile (or reuse), borrow a runtime, run,
// and return the raw goja.Value. Public methods wrap this with the
// appropriate unwrap strategy (Export, String, strict assertion, etc.).
func (e *Engine) evalValue(ctx context.Context, expr string, arg any) (goja.Value, error) {
	if err := e.ensureUsable(ctx); err != nil {
		return nil, err
	}

	prog, err := e.programForArg(expr, arg)
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
		// Wrapper synthesis always produces a function expression; this is
		// an internal invariant failure.
		return nil, fmt.Errorf("scripting: compiled wrapper is not callable")
	}

	result, err := fn(goja.Undefined(), vm.ToValue(arg))
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
