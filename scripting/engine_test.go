package scripting

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// cacheLen counts compiled programs currently held by the engine. Only
// used by tests.
func (e *Engine) cacheLen() int {
	n := 0
	e.cache.Range(func(_, _ any) bool {
		n++
		return true
	})
	return n
}

func TestEval_Basics(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	cases := []struct {
		name string
		src  string
		data map[string]any
		want any
	}{
		{
			name: "template literal with one variable",
			src:  "`Hello ${name}`",
			data: map[string]any{"name": "World"},
			want: "Hello World",
		},
		{
			name: "template literal with nested property",
			src:  "`Hello ${user.name}`",
			data: map[string]any{"user": map[string]any{"name": "Alice"}},
			want: "Hello Alice",
		},
		{
			name: "arithmetic expression",
			src:  "x + y",
			data: map[string]any{"x": 2, "y": 3},
			want: int64(5),
		},
		{
			name: "object literal return",
			src:  "({a: x, b: y})",
			data: map[string]any{"x": 1, "y": 2},
			want: map[string]any{"a": int64(1), "b": int64(2)},
		},
		{
			name: "no data",
			src:  "1 + 1",
			data: nil,
			want: int64(2),
		},
		{
			name: "built-in globals available",
			src:  "Math.max(a, b)",
			data: map[string]any{"a": 3, "b": 7},
			want: int64(7),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := e.Eval(context.Background(), tc.src, tc.data)
			if err != nil {
				t.Fatalf("Eval: unexpected error: %v", err)
			}
			if !equal(got, tc.want) {
				t.Errorf("Eval: got %#v (%T); want %#v (%T)", got, got, tc.want, tc.want)
			}
		})
	}
}

func TestEval_InputBinding(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	got, err := e.Eval(context.Background(), "input + 1", 41)
	if err != nil {
		t.Fatalf("Eval: unexpected error: %v", err)
	}
	if got != int64(42) {
		t.Fatalf("Eval: got %#v; want %#v", got, int64(42))
	}

	got, err = e.Eval(context.Background(), "typeof name", "Alice")
	if err != nil {
		t.Fatalf("Eval: unexpected error: %v", err)
	}
	if got != "undefined" {
		t.Fatalf("Eval: got %#v; want %#v", got, "undefined")
	}

	got, err = e.Eval(context.Background(), "input === null", nil)
	if err != nil {
		t.Fatalf("Eval: unexpected error: %v", err)
	}
	if got != true {
		t.Fatalf("Eval: got %#v; want %#v", got, true)
	}
}

func TestEval_MapBindingDoesNotExposeInput(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	got, err := e.Eval(context.Background(), "typeof input", map[string]any{"name": "Alice"})
	if err != nil {
		t.Fatalf("Eval: unexpected error: %v", err)
	}
	if got != "undefined" {
		t.Fatalf("Eval: got %#v; want %#v", got, "undefined")
	}
}

func TestEvalString_Coercion(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	cases := []struct {
		name string
		src  string
		data map[string]any
		want string
	}{
		{
			name: "template literal",
			src:  "`Hello ${name}`",
			data: map[string]any{"name": "World"},
			want: "Hello World",
		},
		{
			name: "bare string identifier",
			src:  "name",
			data: map[string]any{"name": "Alice"},
			want: "Alice",
		},
		{
			name: "number coerces to decimal string",
			src:  "age",
			data: map[string]any{"age": 42},
			want: "42",
		},
		{
			name: "boolean coerces",
			src:  "true",
			data: nil,
			want: "true",
		},
		{
			name: "null coerces",
			src:  "null",
			data: nil,
			want: "null",
		},
		{
			name: "undefined coerces",
			src:  "undefined",
			data: nil,
			want: "undefined",
		},
		{
			name: "plain object coerces",
			src:  "({a: 1})",
			data: nil,
			want: "[object Object]",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := e.EvalString(context.Background(), tc.src, tc.data)
			if err != nil {
				t.Fatalf("EvalString: unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("EvalString: got %q; want %q", got, tc.want)
			}
		})
	}
}

func TestEvalStringStrict(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	t.Run("accepts strings", func(t *testing.T) {
		got, err := e.EvalStringStrict(context.Background(),
			"`Hello ${name}`", map[string]any{"name": "World"})
		if err != nil {
			t.Fatalf("EvalStringStrict: unexpected error: %v", err)
		}
		if got != "Hello World" {
			t.Errorf("EvalStringStrict: got %q; want %q", got, "Hello World")
		}
	})

	nonStrings := []struct {
		name string
		src  string
	}{
		{"number", "42"},
		{"boolean", "true"},
		{"null", "null"},
		{"undefined", "undefined"},
		{"object", "({a: 1})"},
	}
	for _, tc := range nonStrings {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			_, err := e.EvalStringStrict(context.Background(), tc.src, nil)
			if err == nil {
				t.Fatalf("EvalStringStrict: expected error for %s result, got nil", tc.name)
			}
			if !strings.Contains(err.Error(), "EvalStringStrict") {
				t.Errorf("EvalStringStrict: error should name the method: %v", err)
			}
		})
	}
}

func TestEvalString_SharesCacheWithEval(t *testing.T) {
	// All three methods go through the same programFor(), so the cache
	// should be shared. Two calls with the same src/shape via different
	// methods should produce exactly one cache entry.
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	src := "`Hello ${name}`"
	data := map[string]any{"name": "World"}

	if _, err := e.Eval(context.Background(), src, data); err != nil {
		t.Fatal(err)
	}
	if _, err := e.EvalString(context.Background(), src, data); err != nil {
		t.Fatal(err)
	}
	if _, err := e.EvalStringStrict(context.Background(), src, data); err != nil {
		t.Fatal(err)
	}

	if got := e.cacheLen(); got != 1 {
		t.Errorf("cacheLen: got %d; want 1 (Eval/EvalString/EvalStringStrict must share the cache)", got)
	}
}

func TestEval_CompileError(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	_, err := e.Eval(context.Background(), "`unterminated", nil)
	if err == nil {
		t.Fatal("Eval: expected compile error, got nil")
	}
	if !strings.Contains(err.Error(), "compile") {
		t.Errorf("Eval: error should mention compile step: %v", err)
	}
}

func TestEval_RuntimeError(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	// Referencing a property on undefined throws at runtime.
	_, err := e.Eval(context.Background(), "missing.foo", nil)
	if err == nil {
		t.Fatal("Eval: expected runtime error, got nil")
	}
}

func TestEval_ClosedEngine(t *testing.T) {
	e := NewEngine(Options{})
	e.Close()

	_, err := e.Eval(context.Background(), "1", nil)
	if !errors.Is(err, ErrClosed) {
		t.Errorf("Eval after Close: got %v; want ErrClosed", err)
	}
}

func TestEval_ContextCancellation(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after a short delay; the infinite loop must yield to Interrupt.
	go func() {
		time.Sleep(25 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := e.Eval(ctx, "(()=>{while(true){}})()", nil)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Eval: expected interrupt error, got nil")
	}
	if !errors.Is(err, ErrInterrupted) {
		t.Errorf("Eval: error should wrap ErrInterrupted: %v", err)
	}
	if elapsed > time.Second {
		t.Errorf("Eval: took %v; watchdog should have interrupted well under 1s", elapsed)
	}
}

func TestEval_EvalTimeout(t *testing.T) {
	e := NewEngine(Options{EvalTimeout: 25 * time.Millisecond})
	t.Cleanup(func() { e.Close() })

	start := time.Now()
	_, err := e.Eval(context.Background(), "(()=>{while(true){}})()", nil)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Eval: expected interrupt error, got nil")
	}
	if !errors.Is(err, ErrInterrupted) {
		t.Errorf("Eval: error should wrap ErrInterrupted: %v", err)
	}
	if elapsed > time.Second {
		t.Errorf("Eval: took %v; EvalTimeout should have fired well under 1s", elapsed)
	}
}

func TestFunctionTimeout_DoesNotPoisonLaterEval(t *testing.T) {
	old := runtime.GOMAXPROCS(1)
	t.Cleanup(func() { runtime.GOMAXPROCS(old) })

	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	fn, err := e.FunctionTimeout("1 + 1", nil, 25*time.Millisecond)
	if err != nil {
		t.Fatalf("FunctionTimeout: unexpected error: %v", err)
	}

	for i := 0; i < 1000; i++ {
		got, err := fn(nil)
		if err != nil {
			t.Fatalf("FunctionTimeout iteration %d: unexpected error: %v", i, err)
		}
		if got != int64(2) {
			t.Fatalf("FunctionTimeout iteration %d: got %#v; want %#v", i, got, int64(2))
		}

		got, err = e.Eval(context.Background(), "1 + 1", nil)
		if err != nil {
			t.Fatalf("Eval iteration %d: unexpected error after FunctionTimeout call: %v", i, err)
		}
		if got != int64(2) {
			t.Fatalf("Eval iteration %d: got %#v; want %#v", i, got, int64(2))
		}
	}
}

func TestEval_DataLifetime(t *testing.T) {
	// Pin to a single OS thread so repeated Eval calls reuse the same
	// pooled runtime with high probability.
	old := runtime.GOMAXPROCS(1)
	t.Cleanup(func() { runtime.GOMAXPROCS(old) })

	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	// Prime a pooled runtime with data containing `user`.
	for i := 0; i < 100; i++ {
		_, err := e.Eval(context.Background(), "`Hi ${user.name}`",
			map[string]any{"user": map[string]any{"name": "Alice"}})
		if err != nil {
			t.Fatalf("prime: %v", err)
		}
	}

	// Now call without any data: `user` must not be visible as a global.
	// `typeof` on an undeclared identifier yields "undefined" without
	// throwing, which is the cleanest way to assert non-presence.
	got, err := e.Eval(context.Background(), "typeof user", nil)
	if err != nil {
		t.Fatalf("typeof user: %v", err)
	}
	if got != "undefined" {
		t.Errorf("typeof user: got %v; want \"undefined\" (global leaked from prior call)", got)
	}
}

func TestEval_CacheReuseSameShape(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	src := "`Hi ${user.name}`"
	for _, name := range []string{"Alice", "Bob", "Zara", "Eve"} {
		_, err := e.Eval(context.Background(), src,
			map[string]any{"user": map[string]any{"name": name}})
		if err != nil {
			t.Fatalf("Eval %q: %v", name, err)
		}
	}

	if got := e.cacheLen(); got != 1 {
		t.Errorf("cacheLen: got %d; want 1 (same src + shape)", got)
	}
}

func TestEval_CacheReuseInputShape(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	src := "input"

	if _, err := e.Eval(context.Background(), src, "Alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Eval(context.Background(), src, 42); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Eval(context.Background(), src, nil); err != nil {
		t.Fatal(err)
	}

	if got := e.cacheLen(); got != 1 {
		t.Errorf("cacheLen: got %d; want 1 (all non-map args share the input shape)", got)
	}
}

func TestEval_CacheDiffersByShape(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	src := "`Hi ${user.name}`"

	if _, err := e.Eval(context.Background(), src,
		map[string]any{"user": map[string]any{"name": "A"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Eval(context.Background(), src,
		map[string]any{
			"user": map[string]any{"name": "A"},
			"site": map[string]any{"title": "S"},
		}); err != nil {
		t.Fatal(err)
	}

	if got := e.cacheLen(); got != 2 {
		t.Errorf("cacheLen: got %d; want 2 (two distinct shapes)", got)
	}
}

func TestEval_ConcurrentNoLeakage(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	const goroutines = 16
	const iterations = 200

	var wg sync.WaitGroup
	wg.Add(goroutines)
	errs := make(chan error, goroutines*iterations)

	for g := 0; g < goroutines; g++ {
		go func(g int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				name := fmt.Sprintf("g%d-i%d", g, i)
				got, err := e.Eval(context.Background(),
					"`hi ${user.name}`",
					map[string]any{"user": map[string]any{"name": name}})
				if err != nil {
					errs <- fmt.Errorf("g%d i%d: %w", g, i, err)
					return
				}
				if got != "hi "+name {
					errs <- fmt.Errorf("g%d i%d: got %v; want %q", g, i, got, "hi "+name)
					return
				}
			}
		}(g)
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestEval_SandboxDefaults(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	// The hardened runtime still does not provide these Node-style globals.
	// We check them via `typeof` so the runtime doesn't throw.
	forbidden := []string{"require", "process", "console", "setTimeout", "setInterval", "Buffer"}
	for _, name := range forbidden {
		got, err := e.Eval(context.Background(), "typeof "+name, nil)
		if err != nil {
			t.Fatalf("typeof %s: %v", name, err)
		}
		if got != "undefined" {
			t.Errorf("typeof %s: got %v; want \"undefined\"", name, got)
		}
	}
}

func TestEval_GlobalHardeningBlocksBroadSideEffects(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	cases := []struct {
		name string
		expr string
	}{
		{
			name: "global object extension",
			expr: "globalThis.shared = 1",
		},
		{
			name: "global constructor reassignment",
			expr: "Object = 123",
		},
		{
			name: "object prototype extension",
			expr: "Object.prototype.shared = 1",
		},
		{
			name: "array prototype mutation",
			expr: "Array.prototype.push = function() { return 0 }",
		},
		{
			name: "function prototype mutation",
			expr: "Function.prototype.call = function() { return 0 }",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := e.Eval(context.Background(), tc.expr, nil)
			if err == nil {
				t.Fatal("Eval: expected hardening error, got nil")
			}
			if !strings.Contains(err.Error(), "TypeError") {
				t.Fatalf("Eval: got %q; want TypeError", err)
			}
		})
	}
}

func TestEval_GlobalHardeningPreservesBuiltins(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	got, err := e.Eval(context.Background(), "[1, 2, 3].map(n => n + 1).join(',')", nil)
	if err != nil {
		t.Fatalf("Eval: unexpected error: %v", err)
	}
	if got != "2,3,4" {
		t.Fatalf("Eval: got %#v; want %#v", got, "2,3,4")
	}

	got, err = e.Eval(context.Background(), "Object.keys({a: 1, b: 2}).join(',')", nil)
	if err != nil {
		t.Fatalf("Eval: unexpected error: %v", err)
	}
	if got != "a,b" {
		t.Fatalf("Eval: got %#v; want %#v", got, "a,b")
	}
}

func TestEval_MapBindingRejectsInvalidTopLevelKeys(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	_, err := e.Eval(context.Background(), "`hi`", map[string]any{
		"default":   "value",
		"full-name": "Alice",
	})
	if err == nil {
		t.Fatal("Eval: expected invalid key error, got nil")
	}
	if !strings.Contains(err.Error(), `"default"`) || !strings.Contains(err.Error(), `"full-name"`) {
		t.Fatalf("Eval: got %q; want both invalid keys named", err)
	}
	if !strings.Contains(err.Error(), "strict-mode JavaScript binding names") {
		t.Fatalf("Eval: got %q; want actionable binding-name error", err)
	}
}

func TestEval_NoGlobalLeakFromScript(t *testing.T) {
	old := runtime.GOMAXPROCS(1)
	t.Cleanup(func() { runtime.GOMAXPROCS(old) })

	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	// "use strict" + function scope prevent top-level `var` and bare
	// assignment from creating globals.
	_, err := e.Eval(context.Background(), "(function(){ var leaked = 1; return leaked; })()", nil)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	got, err := e.Eval(context.Background(), "typeof leaked", nil)
	if err != nil {
		t.Fatalf("typeof leaked: %v", err)
	}
	if got != "undefined" {
		t.Errorf("typeof leaked: got %v; want \"undefined\"", got)
	}
}

func TestFunctionContext_MapBindingContract(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	fn, err := e.FunctionContext("typeof site", map[string]any{"user": nil})
	if err != nil {
		t.Fatalf("FunctionContext: unexpected error: %v", err)
	}

	got, err := fn(context.Background(), map[string]any{
		"user": map[string]any{"name": "Alice"},
		"site": "example.com",
	})
	if err != nil {
		t.Fatalf("FunctionContext: unexpected error: %v", err)
	}
	if got != "undefined" {
		t.Fatalf("FunctionContext: got %#v; want %#v", got, "undefined")
	}

	_, err = fn(context.Background(), "not a map")
	if err == nil {
		t.Fatal("FunctionContext: expected type validation error, got nil")
	}
	if !strings.Contains(err.Error(), "map[string]any") {
		t.Fatalf("FunctionContext: got %q; want map validation error", err)
	}

	fn, err = e.FunctionContext("user.name", map[string]any{"user": nil, "site": nil})
	if err != nil {
		t.Fatalf("FunctionContext: unexpected error: %v", err)
	}

	_, err = fn(context.Background(), map[string]any{"user": map[string]any{"name": "Alice"}})
	if err == nil {
		t.Fatal("FunctionContext: expected missing key validation error, got nil")
	}
	if !strings.Contains(err.Error(), "site") {
		t.Fatalf("FunctionContext: got %q; want missing key error", err)
	}
}

func TestFunctionContext_InputBindingContract(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	fn, err := e.FunctionContext("typeof name", "")
	if err != nil {
		t.Fatalf("FunctionContext: unexpected error: %v", err)
	}

	got, err := fn(context.Background(), "Alice")
	if err != nil {
		t.Fatalf("FunctionContext: unexpected error: %v", err)
	}
	if got != "undefined" {
		t.Fatalf("FunctionContext: got %#v; want %#v", got, "undefined")
	}

	fn, err = e.FunctionContext("input + 1", 0)
	if err != nil {
		t.Fatalf("FunctionContext: unexpected error: %v", err)
	}

	got, err = fn(context.Background(), 41)
	if err != nil {
		t.Fatalf("FunctionContext: unexpected error: %v", err)
	}
	if got != int64(42) {
		t.Fatalf("FunctionContext: got %#v; want %#v", got, int64(42))
	}

	_, err = fn(context.Background(), map[string]any{"input": 41})
	if err == nil {
		t.Fatal("FunctionContext: expected map rejection, got nil")
	}
	if !strings.Contains(err.Error(), "non-map input") {
		t.Fatalf("FunctionContext: got %q; want non-map validation error", err)
	}
}

func TestFunctionContext_RespectsCancellation(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	fn, err := e.FunctionContext("(()=>{while(true){}})()", nil)
	if err != nil {
		t.Fatalf("FunctionContext: unexpected error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(25 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err = fn(ctx, nil)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("FunctionContext: expected interrupt error, got nil")
	}
	if !errors.Is(err, ErrInterrupted) {
		t.Fatalf("FunctionContext: error should wrap ErrInterrupted: %v", err)
	}
	if elapsed > time.Second {
		t.Fatalf("FunctionContext: took %v; cancellation should have interrupted well under 1s", elapsed)
	}
}

func TestFunctionTimeout_HonorsTimeout(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	fn, err := e.FunctionTimeout("(()=>{while(true){}})()", nil, 25*time.Millisecond)
	if err != nil {
		t.Fatalf("FunctionTimeout: unexpected error: %v", err)
	}

	start := time.Now()
	_, err = fn(nil)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("FunctionTimeout: expected interrupt error, got nil")
	}
	if !errors.Is(err, ErrInterrupted) {
		t.Fatalf("FunctionTimeout: error should wrap ErrInterrupted: %v", err)
	}
	if elapsed > time.Second {
		t.Fatalf("FunctionTimeout: took %v; timeout should have interrupted well under 1s", elapsed)
	}
}

func TestFunction_UsesDefaultTimeout(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	fn, err := e.Function("(()=>{while(true){}})()", nil)
	if err != nil {
		t.Fatalf("Function: unexpected error: %v", err)
	}

	start := time.Now()
	_, err = fn(nil)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Function: expected interrupt error, got nil")
	}
	if !errors.Is(err, ErrInterrupted) {
		t.Fatalf("Function: error should wrap ErrInterrupted: %v", err)
	}
	if elapsed < 150*time.Millisecond {
		t.Fatalf("Function: took %v; default timeout should be substantially above immediate cancellation", elapsed)
	}
	if elapsed > time.Second {
		t.Fatalf("Function: took %v; default timeout should have interrupted well under 1s", elapsed)
	}
}

func TestFunction_DefaultEngineDelegatesAndReusesSingleton(t *testing.T) {
	engine := shared()
	before := engine.cacheLen()

	expr := fmt.Sprintf("`%s ${name}`", t.Name())
	fn, err := Function(expr, map[string]any{"name": nil})
	if err != nil {
		t.Fatalf("Function: unexpected error: %v", err)
	}

	got, err := fn(map[string]any{"name": "Alice"})
	if err != nil {
		t.Fatalf("Function: unexpected error: %v", err)
	}
	if got != t.Name()+" Alice" {
		t.Fatalf("Function: got %#v; want %#v", got, t.Name()+" Alice")
	}

	if shared() != engine {
		t.Fatal("shared: expected singleton engine pointer")
	}

	if got := shared().cacheLen(); got != before+1 {
		t.Fatalf("cacheLen: got %d; want %d after package-level Function call", got, before+1)
	}
}

func TestFunctionContextAndFunctionTimeout_DefaultEngineDelegates(t *testing.T) {
	engine := shared()
	before := engine.cacheLen()

	exprContext := fmt.Sprintf("`%s ${name}`", t.Name())
	fnContext, err := FunctionContext(exprContext, map[string]any{"name": nil})
	if err != nil {
		t.Fatalf("FunctionContext: unexpected error: %v", err)
	}

	got, err := fnContext(context.Background(), map[string]any{"name": "Alice"})
	if err != nil {
		t.Fatalf("FunctionContext: unexpected error: %v", err)
	}
	if got != t.Name()+" Alice" {
		t.Fatalf("FunctionContext: got %#v; want %#v", got, t.Name()+" Alice")
	}

	exprTimeout := fmt.Sprintf("`%s ${input}`", t.Name())
	fnTimeout, err := FunctionTimeout(exprTimeout, "", 100*time.Millisecond)
	if err != nil {
		t.Fatalf("FunctionTimeout: unexpected error: %v", err)
	}

	got, err = fnTimeout("Bob")
	if err != nil {
		t.Fatalf("FunctionTimeout: unexpected error: %v", err)
	}
	if got != t.Name()+" Bob" {
		t.Fatalf("FunctionTimeout: got %#v; want %#v", got, t.Name()+" Bob")
	}

	if shared() != engine {
		t.Fatal("shared: expected singleton engine pointer")
	}

	if got := shared().cacheLen(); got != before+2 {
		t.Fatalf("cacheLen: got %d; want %d after FunctionContext and FunctionTimeout", got, before+2)
	}
}

func TestFunction_SharesCacheWithEval(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	src := fmt.Sprintf("`%s ${user.name}`", t.Name())
	before := e.cacheLen()

	fn, err := e.FunctionContext(src, map[string]any{"user": nil})
	if err != nil {
		t.Fatalf("FunctionContext: unexpected error: %v", err)
	}

	if _, err := fn(context.Background(), map[string]any{"user": map[string]any{"name": "Alice"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Eval(context.Background(), src, map[string]any{"user": map[string]any{"name": "Bob"}}); err != nil {
		t.Fatal(err)
	}

	if got := e.cacheLen(); got != before+1 {
		t.Fatalf("cacheLen: got %d; want %d (FunctionContext and Eval should share the cache)", got, before+1)
	}
}

// equal compares two values for equality, including nested maps/slices
// emitted by goja's Export(). Falls back to fmt.Sprintf for anything else.
func equal(a, b any) bool {
	switch av := a.(type) {
	case map[string]any:
		bv, ok := b.(map[string]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for k, v := range av {
			if !equal(v, bv[k]) {
				return false
			}
		}
		return true
	case []any:
		bv, ok := b.([]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !equal(av[i], bv[i]) {
				return false
			}
		}
		return true
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}
