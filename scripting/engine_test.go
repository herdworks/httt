package scripting

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
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

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}

	os.Stdout = writer
	output := make(chan string, 1)
	go func() {
		var b strings.Builder
		_, _ = io.Copy(&b, reader)
		output <- b.String()
	}()

	fn()

	os.Stdout = old
	if err := writer.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	got := <-output
	if err := reader.Close(); err != nil {
		t.Fatalf("close stdout reader: %v", err)
	}
	return got
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
			src:  "return `Hello ${name}`",
			data: map[string]any{"name": "World"},
			want: "Hello World",
		},
		{
			name: "template literal with nested property",
			src:  "return `Hello ${user.name}`",
			data: map[string]any{"user": map[string]any{"name": "Alice"}},
			want: "Hello Alice",
		},
		{
			name: "arithmetic return",
			src:  "return x + y",
			data: map[string]any{"x": 2, "y": 3},
			want: int64(5),
		},
		{
			name: "object literal return",
			src:  "return {a: x, b: y}",
			data: map[string]any{"x": 1, "y": 2},
			want: map[string]any{"a": int64(1), "b": int64(2)},
		},
		{
			name: "no data",
			src:  "return 1 + 1",
			data: nil,
			want: int64(2),
		},
		{
			name: "built-in globals available",
			src:  "return Math.max(a, b)",
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

	got, err := e.Eval(context.Background(), "return input + 1", 41)
	if err != nil {
		t.Fatalf("Eval: unexpected error: %v", err)
	}
	if got != int64(42) {
		t.Fatalf("Eval: got %#v; want %#v", got, int64(42))
	}

	got, err = e.Eval(context.Background(), "return typeof name", "Alice")
	if err != nil {
		t.Fatalf("Eval: unexpected error: %v", err)
	}
	if got != "undefined" {
		t.Fatalf("Eval: got %#v; want %#v", got, "undefined")
	}

	got, err = e.Eval(context.Background(), "return input === null", nil)
	if err != nil {
		t.Fatalf("Eval: unexpected error: %v", err)
	}
	if got != true {
		t.Fatalf("Eval: got %#v; want %#v", got, true)
	}
}

func TestEval_StatementBody(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	src := `
		if (typeof input !== "string") {
			return input
		}
		return input.toUpperCase()
	`

	got, err := e.Eval(context.Background(), src, "hello")
	if err != nil {
		t.Fatalf("Eval: unexpected error: %v", err)
	}
	if got != "HELLO" {
		t.Fatalf("Eval: got %#v; want %#v", got, "HELLO")
	}

	got, err = e.Eval(context.Background(), src, 42)
	if err != nil {
		t.Fatalf("Eval: unexpected error: %v", err)
	}
	if got != int64(42) {
		t.Fatalf("Eval: got %#v; want %#v", got, int64(42))
	}
}

func TestEval_NoReturnExportsNil(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	got, err := e.Eval(context.Background(), "const value = input", "hello")
	if err != nil {
		t.Fatalf("Eval: unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("Eval: got %#v; want nil", got)
	}
}

func TestEval_MapBindingDoesNotExposeInput(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	got, err := e.Eval(context.Background(), "return typeof input", map[string]any{"name": "Alice"})
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
			src:  "return `Hello ${name}`",
			data: map[string]any{"name": "World"},
			want: "Hello World",
		},
		{
			name: "bare string identifier",
			src:  "return name",
			data: map[string]any{"name": "Alice"},
			want: "Alice",
		},
		{
			name: "number coerces to decimal string",
			src:  "return age",
			data: map[string]any{"age": 42},
			want: "42",
		},
		{
			name: "boolean coerces",
			src:  "return true",
			data: nil,
			want: "true",
		},
		{
			name: "null coerces",
			src:  "return null",
			data: nil,
			want: "null",
		},
		{
			name: "undefined coerces",
			src:  "return undefined",
			data: nil,
			want: "undefined",
		},
		{
			name: "plain object coerces",
			src:  "return {a: 1}",
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
			"return `Hello ${name}`", map[string]any{"name": "World"})
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
		{"number", "return 42"},
		{"boolean", "return true"},
		{"null", "return null"},
		{"undefined", "return undefined"},
		{"object", "return {a: 1}"},
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

	t.Run("rejects no return", func(t *testing.T) {
		_, err := e.EvalStringStrict(context.Background(), "const value = 'hello'", nil)
		if err == nil {
			t.Fatal("EvalStringStrict: expected error for no return, got nil")
		}
		if !strings.Contains(err.Error(), "EvalStringStrict") {
			t.Errorf("EvalStringStrict: error should name the method: %v", err)
		}
	})
}

func TestEvalString_SharesCacheWithEval(t *testing.T) {
	// All three methods go through the same programFor(), so the cache
	// should be shared. Two calls with the same src/shape via different
	// methods should produce exactly one cache entry.
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	src := "return `Hello ${name}`"
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

	_, err := e.Eval(context.Background(), "return `unterminated", nil)
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
	_, err := e.Eval(context.Background(), "return missing.foo", nil)
	if err == nil {
		t.Fatal("Eval: expected runtime error, got nil")
	}
}

func TestEval_ClosedEngine(t *testing.T) {
	e := NewEngine(Options{})
	e.Close()

	_, err := e.Eval(context.Background(), "return 1", nil)
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
	_, err := e.Eval(ctx, "while (true) {}", nil)
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
	_, err := e.Eval(context.Background(), "while (true) {}", nil)
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

func TestScriptTimeout_DoesNotPoisonLaterEval(t *testing.T) {
	old := runtime.GOMAXPROCS(1)
	t.Cleanup(func() { runtime.GOMAXPROCS(old) })

	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	fn, err := e.ScriptTimeout("return 1 + 1", nil, 25*time.Millisecond)
	if err != nil {
		t.Fatalf("ScriptTimeout: unexpected error: %v", err)
	}

	for i := 0; i < 1000; i++ {
		got, err := fn(nil)
		if err != nil {
			t.Fatalf("ScriptTimeout iteration %d: unexpected error: %v", i, err)
		}
		if got != int64(2) {
			t.Fatalf("ScriptTimeout iteration %d: got %#v; want %#v", i, got, int64(2))
		}

		got, err = e.Eval(context.Background(), "return 1 + 1", nil)
		if err != nil {
			t.Fatalf("Eval iteration %d: unexpected error after ScriptTimeout call: %v", i, err)
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
		_, err := e.Eval(context.Background(), "return `Hi ${user.name}`",
			map[string]any{"user": map[string]any{"name": "Alice"}})
		if err != nil {
			t.Fatalf("prime: %v", err)
		}
	}

	// Now call without any data: `user` must not be visible as a global.
	// `typeof` on an undeclared identifier yields "undefined" without
	// throwing, which is the cleanest way to assert non-presence.
	got, err := e.Eval(context.Background(), "return typeof user", nil)
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

	src := "return `Hi ${user.name}`"
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

	src := "return input"

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

	src := "return `Hi ${user.name}`"

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
					"return `hi ${user.name}`",
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
	forbidden := []string{"require", "process", "setTimeout", "setInterval", "Buffer"}
	for _, name := range forbidden {
		got, err := e.Eval(context.Background(), "return typeof "+name, nil)
		if err != nil {
			t.Fatalf("typeof %s: %v", name, err)
		}
		if got != "undefined" {
			t.Errorf("typeof %s: got %v; want \"undefined\"", name, got)
		}
	}
}

func TestEval_ConsoleWritesScriptingPrefixedLogs(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	var got any
	var err error
	output := captureStdout(t, func() {
		got, err = e.Eval(context.Background(), `
			console.log("hello", input)
			console.info("ready")
			console.warn("careful")
			console.error("broken")
			console.debug("details")
			return input
		`, "world")
	})
	if err != nil {
		t.Fatalf("Eval: unexpected error: %v", err)
	}
	if got != "world" {
		t.Fatalf("Eval: got %#v; want %#v", got, "world")
	}

	want := strings.Join([]string{
		"[scripting] hello world",
		"[scripting] INFO ready",
		"[scripting] WARN careful",
		"[scripting] ERROR broken",
		"[scripting] DEBUG details",
		"",
	}, "\n")
	if output != want {
		t.Fatalf("stdout:\ngot:\n%swant:\n%s", output, want)
	}
}

func TestEval_ConsoleLoggerWritesNamedPrefixedLogs(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	var got any
	var err error
	output := captureStdout(t, func() {
		got, err = e.Eval(context.Background(), `
			const logger = console.logger("plugin/foo")
			logger.log("hello", input)
			logger.warn("careful")
			console.log("still default")
			return logger.name
		`, "world")
	})
	if err != nil {
		t.Fatalf("Eval: unexpected error: %v", err)
	}
	if got != "plugin/foo" {
		t.Fatalf("Eval: got %#v; want %#v", got, "plugin/foo")
	}

	want := strings.Join([]string{
		"[plugin/foo] hello world",
		"[plugin/foo] WARN careful",
		"[scripting] still default",
		"",
	}, "\n")
	if output != want {
		t.Fatalf("stdout:\ngot:\n%swant:\n%s", output, want)
	}
}

func TestSetConsoleOutputRedirectsAndSilences(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })
	t.Cleanup(func() { SetConsoleOutput(nil) }) // restore default

	// Custom writer captures script output instead of stdout.
	var buf bytes.Buffer
	SetConsoleOutput(&buf)
	stdoutDuringRedirect := captureStdout(t, func() {
		if _, err := e.Eval(context.Background(), `
			console.log("redirected", input)
			return null
		`, "value"); err != nil {
			t.Fatalf("Eval: %v", err)
		}
	})
	if stdoutDuringRedirect != "" {
		t.Fatalf("stdout should be empty when redirected, got: %q", stdoutDuringRedirect)
	}
	if got, want := buf.String(), "[scripting] redirected value\n"; got != want {
		t.Fatalf("buf = %q; want %q", got, want)
	}

	// io.Discard silences output entirely.
	SetConsoleOutput(io.Discard)
	stdoutDuringDiscard := captureStdout(t, func() {
		if _, err := e.Eval(context.Background(), `
			console.log("dropped")
			return null
		`, "value"); err != nil {
			t.Fatalf("Eval: %v", err)
		}
	})
	if stdoutDuringDiscard != "" {
		t.Fatalf("stdout should be empty when discarded, got: %q", stdoutDuringDiscard)
	}

	// Resetting to nil restores os.Stdout.
	SetConsoleOutput(nil)
	stdoutAfterReset := captureStdout(t, func() {
		if _, err := e.Eval(context.Background(), `
			console.log("restored")
			return null
		`, "value"); err != nil {
			t.Fatalf("Eval: %v", err)
		}
	})
	if got, want := stdoutAfterReset, "[scripting] restored\n"; got != want {
		t.Fatalf("stdout = %q; want %q", got, want)
	}
}

func TestEval_GlobalHardeningBlocksBroadSideEffects(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	cases := []struct {
		name string
		src  string
	}{
		{
			name: "global object extension",
			src:  "globalThis.shared = 1",
		},
		{
			name: "global constructor reassignment",
			src:  "Object = 123",
		},
		{
			name: "object prototype extension",
			src:  "Object.prototype.shared = 1",
		},
		{
			name: "array prototype mutation",
			src:  "Array.prototype.push = function() { return 0 }",
		},
		{
			name: "function prototype mutation",
			src:  "Function.prototype.call = function() { return 0 }",
		},
		{
			name: "console mutation",
			src:  "console.log = function() {}",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := e.Eval(context.Background(), tc.src, nil)
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

	got, err := e.Eval(context.Background(), "return [1, 2, 3].map(n => n + 1).join(',')", nil)
	if err != nil {
		t.Fatalf("Eval: unexpected error: %v", err)
	}
	if got != "2,3,4" {
		t.Fatalf("Eval: got %#v; want %#v", got, "2,3,4")
	}

	got, err = e.Eval(context.Background(), "return Object.keys({a: 1, b: 2}).join(',')", nil)
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

	_, err := e.Eval(context.Background(), "return `hi`", map[string]any{
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
	_, err := e.Eval(context.Background(), "var leaked = 1; return leaked", nil)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	got, err := e.Eval(context.Background(), "return typeof leaked", nil)
	if err != nil {
		t.Fatalf("typeof leaked: %v", err)
	}
	if got != "undefined" {
		t.Errorf("typeof leaked: got %v; want \"undefined\"", got)
	}
}

func TestScriptContext_MapBindingContract(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	fn, err := e.ScriptContext("return typeof site", map[string]any{"user": nil})
	if err != nil {
		t.Fatalf("ScriptContext: unexpected error: %v", err)
	}

	got, err := fn(context.Background(), map[string]any{
		"user": map[string]any{"name": "Alice"},
		"site": "example.com",
	})
	if err != nil {
		t.Fatalf("ScriptContext: unexpected error: %v", err)
	}
	if got != "undefined" {
		t.Fatalf("ScriptContext: got %#v; want %#v", got, "undefined")
	}

	_, err = fn(context.Background(), "not a map")
	if err == nil {
		t.Fatal("ScriptContext: expected type validation error, got nil")
	}
	if !strings.Contains(err.Error(), "map[string]any") {
		t.Fatalf("ScriptContext: got %q; want map validation error", err)
	}

	fn, err = e.ScriptContext("return user.name", map[string]any{"user": nil, "site": nil})
	if err != nil {
		t.Fatalf("ScriptContext: unexpected error: %v", err)
	}

	_, err = fn(context.Background(), map[string]any{"user": map[string]any{"name": "Alice"}})
	if err == nil {
		t.Fatal("ScriptContext: expected missing key validation error, got nil")
	}
	if !strings.Contains(err.Error(), "site") {
		t.Fatalf("ScriptContext: got %q; want missing key error", err)
	}
}

func TestScriptContext_InputBindingContract(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	fn, err := e.ScriptContext("return typeof name", "")
	if err != nil {
		t.Fatalf("ScriptContext: unexpected error: %v", err)
	}

	got, err := fn(context.Background(), "Alice")
	if err != nil {
		t.Fatalf("ScriptContext: unexpected error: %v", err)
	}
	if got != "undefined" {
		t.Fatalf("ScriptContext: got %#v; want %#v", got, "undefined")
	}

	fn, err = e.ScriptContext("return input + 1", 0)
	if err != nil {
		t.Fatalf("ScriptContext: unexpected error: %v", err)
	}

	got, err = fn(context.Background(), 41)
	if err != nil {
		t.Fatalf("ScriptContext: unexpected error: %v", err)
	}
	if got != int64(42) {
		t.Fatalf("ScriptContext: got %#v; want %#v", got, int64(42))
	}

	_, err = fn(context.Background(), map[string]any{"input": 41})
	if err == nil {
		t.Fatal("ScriptContext: expected map rejection, got nil")
	}
	if !strings.Contains(err.Error(), "non-map input") {
		t.Fatalf("ScriptContext: got %q; want non-map validation error", err)
	}

	_, err = fn(context.Background(), testTemplateData{"input": 41})
	if err == nil {
		t.Fatal("ScriptContext: expected defined map rejection, got nil")
	}
	if !strings.Contains(err.Error(), "non-map input") {
		t.Fatalf("ScriptContext: got %q; want non-map validation error", err)
	}
}

func TestScript_NoReturnExportsNil(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	fn, err := e.Script("const value = input", nil)
	if err != nil {
		t.Fatalf("Script: unexpected error: %v", err)
	}

	got, err := fn("hello")
	if err != nil {
		t.Fatalf("Script: unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("Script: got %#v; want nil", got)
	}
}

func TestScriptContext_RespectsCancellation(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	fn, err := e.ScriptContext("while (true) {}", nil)
	if err != nil {
		t.Fatalf("ScriptContext: unexpected error: %v", err)
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
		t.Fatal("ScriptContext: expected interrupt error, got nil")
	}
	if !errors.Is(err, ErrInterrupted) {
		t.Fatalf("ScriptContext: error should wrap ErrInterrupted: %v", err)
	}
	if elapsed > time.Second {
		t.Fatalf("ScriptContext: took %v; cancellation should have interrupted well under 1s", elapsed)
	}
}

func TestScriptTimeout_HonorsTimeout(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	fn, err := e.ScriptTimeout("while (true) {}", nil, 25*time.Millisecond)
	if err != nil {
		t.Fatalf("ScriptTimeout: unexpected error: %v", err)
	}

	start := time.Now()
	_, err = fn(nil)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("ScriptTimeout: expected interrupt error, got nil")
	}
	if !errors.Is(err, ErrInterrupted) {
		t.Fatalf("ScriptTimeout: error should wrap ErrInterrupted: %v", err)
	}
	if elapsed > time.Second {
		t.Fatalf("ScriptTimeout: took %v; timeout should have interrupted well under 1s", elapsed)
	}
}

func TestScript_UsesDefaultTimeout(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	fn, err := e.Script("while (true) {}", nil)
	if err != nil {
		t.Fatalf("Script: unexpected error: %v", err)
	}

	start := time.Now()
	_, err = fn(nil)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Script: expected interrupt error, got nil")
	}
	if !errors.Is(err, ErrInterrupted) {
		t.Fatalf("Script: error should wrap ErrInterrupted: %v", err)
	}
	if elapsed < 150*time.Millisecond {
		t.Fatalf("Script: took %v; default timeout should be substantially above immediate cancellation", elapsed)
	}
	if elapsed > time.Second {
		t.Fatalf("Script: took %v; default timeout should have interrupted well under 1s", elapsed)
	}
}

func TestScript_DefaultEngineDelegatesAndReusesSingleton(t *testing.T) {
	engine := shared()
	before := engine.cacheLen()

	src := fmt.Sprintf("return `%s ${name}`", t.Name())
	fn, err := Script(src, map[string]any{"name": nil})
	if err != nil {
		t.Fatalf("Script: unexpected error: %v", err)
	}

	got, err := fn(map[string]any{"name": "Alice"})
	if err != nil {
		t.Fatalf("Script: unexpected error: %v", err)
	}
	if got != t.Name()+" Alice" {
		t.Fatalf("Script: got %#v; want %#v", got, t.Name()+" Alice")
	}

	if shared() != engine {
		t.Fatal("shared: expected singleton engine pointer")
	}

	if got := shared().cacheLen(); got != before+1 {
		t.Fatalf("cacheLen: got %d; want %d after package-level Script call", got, before+1)
	}
}

func TestScriptContextAndScriptTimeout_DefaultEngineDelegates(t *testing.T) {
	engine := shared()
	before := engine.cacheLen()

	srcContext := fmt.Sprintf("return `%s ${name}`", t.Name())
	fnContext, err := ScriptContext(srcContext, map[string]any{"name": nil})
	if err != nil {
		t.Fatalf("ScriptContext: unexpected error: %v", err)
	}

	got, err := fnContext(context.Background(), map[string]any{"name": "Alice"})
	if err != nil {
		t.Fatalf("ScriptContext: unexpected error: %v", err)
	}
	if got != t.Name()+" Alice" {
		t.Fatalf("ScriptContext: got %#v; want %#v", got, t.Name()+" Alice")
	}

	srcTimeout := fmt.Sprintf("return `%s ${input}`", t.Name())
	fnTimeout, err := ScriptTimeout(srcTimeout, "", 100*time.Millisecond)
	if err != nil {
		t.Fatalf("ScriptTimeout: unexpected error: %v", err)
	}

	got, err = fnTimeout("Bob")
	if err != nil {
		t.Fatalf("ScriptTimeout: unexpected error: %v", err)
	}
	if got != t.Name()+" Bob" {
		t.Fatalf("ScriptTimeout: got %#v; want %#v", got, t.Name()+" Bob")
	}

	if shared() != engine {
		t.Fatal("shared: expected singleton engine pointer")
	}

	if got := shared().cacheLen(); got != before+2 {
		t.Fatalf("cacheLen: got %d; want %d after ScriptContext and ScriptTimeout", got, before+2)
	}
}

func TestScript_SharesCacheWithEval(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	src := fmt.Sprintf("return `%s ${user.name}`", t.Name())
	before := e.cacheLen()

	fn, err := e.ScriptContext(src, map[string]any{"user": nil})
	if err != nil {
		t.Fatalf("ScriptContext: unexpected error: %v", err)
	}

	if _, err := fn(context.Background(), map[string]any{"user": map[string]any{"name": "Alice"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Eval(context.Background(), src, map[string]any{"user": map[string]any{"name": "Bob"}}); err != nil {
		t.Fatal(err)
	}

	if got := e.cacheLen(); got != before+1 {
		t.Fatalf("cacheLen: got %d; want %d (ScriptContext and Eval should share the cache)", got, before+1)
	}
}

func TestFunction_OrderedArgs(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	fn, err := e.Function(`
		function plugin(prefix, input) {
			return prefix + ":" + input
		}
	`)
	if err != nil {
		t.Fatalf("Function: unexpected error: %v", err)
	}

	got, err := fn("title", "Hello")
	if err != nil {
		t.Fatalf("Function: unexpected error: %v", err)
	}
	if got != "title:Hello" {
		t.Fatalf("Function: got %#v; want %#v", got, "title:Hello")
	}
}

func TestFunction_ZeroArgsAndDefaults(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	fn, err := e.Function(`
		function plugin(input = "Untitled") {
			return input
		}
	`)
	if err != nil {
		t.Fatalf("Function: unexpected error: %v", err)
	}

	got, err := fn()
	if err != nil {
		t.Fatalf("Function: unexpected error: %v", err)
	}
	if got != "Untitled" {
		t.Fatalf("Function: got %#v; want %#v", got, "Untitled")
	}
}

func TestFunction_ArrowFunctionEntrypoint(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	fn, err := e.Function(`
		const plugin = (input) => input.toUpperCase()
	`)
	if err != nil {
		t.Fatalf("Function: unexpected error: %v", err)
	}

	got, err := fn("hello")
	if err != nil {
		t.Fatalf("Function: unexpected error: %v", err)
	}
	if got != "HELLO" {
		t.Fatalf("Function: got %#v; want %#v", got, "HELLO")
	}
}

func TestFunction_ConsoleWritesScriptingPrefixedLogs(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	fn, err := e.Function(`
		function plugin(input) {
			console.log("hello", input)
			console.info("ready")
			console.warn("careful")
			console.error("broken")
			console.debug("details")
			return input
		}
	`)
	if err != nil {
		t.Fatalf("Function: unexpected error: %v", err)
	}

	var got any
	output := captureStdout(t, func() {
		got, err = fn("world")
	})
	if err != nil {
		t.Fatalf("Function: unexpected error: %v", err)
	}
	if got != "world" {
		t.Fatalf("Function: got %#v; want %#v", got, "world")
	}

	want := strings.Join([]string{
		"[scripting] hello world",
		"[scripting] INFO ready",
		"[scripting] WARN careful",
		"[scripting] ERROR broken",
		"[scripting] DEBUG details",
		"",
	}, "\n")
	if output != want {
		t.Fatalf("stdout:\ngot:\n%swant:\n%s", output, want)
	}
}

func TestFunction_ConsoleLoggerWritesNamedPrefixedLogs(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	fn, err := e.Function(`
		const logger = console.logger("plugin/trace")

		function plugin(input) {
			logger.log("hello", input)
			logger.error("broken")
			return input
		}
	`)
	if err != nil {
		t.Fatalf("Function: unexpected error: %v", err)
	}

	var got any
	output := captureStdout(t, func() {
		got, err = fn("world")
	})
	if err != nil {
		t.Fatalf("Function: unexpected error: %v", err)
	}
	if got != "world" {
		t.Fatalf("Function: got %#v; want %#v", got, "world")
	}

	want := strings.Join([]string{
		"[plugin/trace] hello world",
		"[plugin/trace] ERROR broken",
		"",
	}, "\n")
	if output != want {
		t.Fatalf("stdout:\ngot:\n%swant:\n%s", output, want)
	}
}

func TestFunction_MapArgIsPlainArgument(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	fn, err := e.Function(`
		function plugin(page) {
			return typeof input + ":" + page.title
		}
	`)
	if err != nil {
		t.Fatalf("Function: unexpected error: %v", err)
	}

	got, err := fn(map[string]any{"title": "Hello"})
	if err != nil {
		t.Fatalf("Function: unexpected error: %v", err)
	}
	if got != "undefined:Hello" {
		t.Fatalf("Function: got %#v; want %#v", got, "undefined:Hello")
	}
}

type testTemplateData map[string]any

func (data testTemplateData) Get(key string) any {
	return data[key]
}

func TestFunction_DefinedStringMapArgIsPlainObject(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	fn, err := e.Function(`
		function plugin(input) {
			return [
				input.title,
				input.nested.ok,
				input.items[0].name,
				JSON.stringify(input.nested),
				Object.keys(input).sort().join(","),
				typeof input.Get
			].join(":")
		}
	`)
	if err != nil {
		t.Fatalf("Function: unexpected error: %v", err)
	}

	got, err := fn(testTemplateData{
		"title":  "Hello",
		"nested": testTemplateData{"ok": true},
		"items":  []testTemplateData{{"name": "First"}},
	})
	if err != nil {
		t.Fatalf("Function: unexpected error: %v", err)
	}
	if got != `Hello:true:First:{"ok":true}:items,nested,title:undefined` {
		t.Fatalf("Function: got %#v; want defined map as plain JavaScript object", got)
	}
}

func TestFunction_NestedDefinedStringMapArgIsPlainObject(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	fn, err := e.Function(`
		function plugin(input) {
			return input.site.title + ":" + Object.keys(input.site).sort().join(",")
		}
	`)
	if err != nil {
		t.Fatalf("Function: unexpected error: %v", err)
	}

	got, err := fn(map[string]any{
		"site": testTemplateData{"title": "Hello"},
	})
	if err != nil {
		t.Fatalf("Function: unexpected error: %v", err)
	}
	if got != "Hello:title" {
		t.Fatalf("Function: got %#v; want nested defined map as plain JavaScript object", got)
	}
}

func TestEval_DefinedStringMapArgUsesMapBinding(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	got, err := e.Eval(context.Background(), "return title", testTemplateData{"title": "Hello"})
	if err != nil {
		t.Fatalf("Eval: unexpected error: %v", err)
	}
	if got != "Hello" {
		t.Fatalf("Eval: got %#v; want %#v", got, "Hello")
	}
}

func TestFunction_RuntimeError(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	fn, err := e.Function(`
		function plugin() {
			return missing.value
		}
	`)
	if err != nil {
		t.Fatalf("Function: unexpected error: %v", err)
	}

	if _, err := fn(); err == nil {
		t.Fatal("Function: expected runtime error, got nil")
	}
}

func TestFunctionContext_RespectsCancellation(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	fn, err := e.FunctionContext(`
		function plugin() {
			while (true) {}
		}
	`)
	if err != nil {
		t.Fatalf("FunctionContext: unexpected error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(25 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err = fn(ctx)
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

	fn, err := e.FunctionTimeout(`
		function plugin() {
			while (true) {}
		}
	`, 25*time.Millisecond)
	if err != nil {
		t.Fatalf("FunctionTimeout: unexpected error: %v", err)
	}

	start := time.Now()
	_, err = fn()
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

func TestFunction_DefaultEngineDelegatesAndReusesSingleton(t *testing.T) {
	engine := shared()
	before := engine.cacheLen()

	src := `
		function plugin(input) {
			return input
		}
	`
	fn, err := Function(src)
	if err != nil {
		t.Fatalf("Function: unexpected error: %v", err)
	}
	if _, err := fn("hello"); err != nil {
		t.Fatalf("Function: unexpected error: %v", err)
	}

	if shared() != engine {
		t.Fatal("shared: expected singleton engine pointer")
	}
	if got := shared().cacheLen(); got != before+1 {
		t.Fatalf("cacheLen: got %d; want %d after package-level Function call", got, before+1)
	}
}

func TestFunction_SharesCacheAcrossCalls(t *testing.T) {
	e := NewEngine(Options{})
	t.Cleanup(func() { e.Close() })

	src := `
		function plugin(input) {
			return input
		}
	`
	before := e.cacheLen()

	fn, err := e.Function(src)
	if err != nil {
		t.Fatalf("Function: unexpected error: %v", err)
	}
	for _, input := range []string{"Alice", "Bob", "Zara"} {
		if _, err := fn(input); err != nil {
			t.Fatalf("Function(%q): %v", input, err)
		}
	}
	if got := e.cacheLen(); got != before+1 {
		t.Fatalf("cacheLen: got %d; want %d after repeated Function calls", got, before+1)
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
