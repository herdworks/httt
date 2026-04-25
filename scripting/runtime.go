package scripting

import (
	"fmt"

	"github.com/dop251/goja"
)

var hardenRuntimeProgram = mustCompileProgram(`
Object.freeze(Object);
Object.freeze(Object.prototype);
Object.freeze(Array);
Object.freeze(Array.prototype);
Object.freeze(Function);
Object.freeze(Function.prototype);
Object.freeze(globalThis);
`)

func newRuntime() *goja.Runtime {
	vm := goja.New()

	if err := hardenRuntime(vm); err != nil {
		panic(fmt.Sprintf("scripting: harden runtime: %v", err))
	}

	return vm
}

// hardenRuntime freezes the highest-blast-radius globals once per pooled
// runtime so scripts cannot leave broad global or prototype mutations behind
// for later evaluations.
func hardenRuntime(vm *goja.Runtime) error {
	if _, err := vm.RunProgram(hardenRuntimeProgram); err != nil {
		return fmt.Errorf("run hardening prelude: %w", err)
	}

	return nil
}

func mustCompileProgram(src string) *goja.Program {
	origin := "runtime.go"
	prog, err := goja.Compile(origin, src, false)
	if err != nil {
		panic(fmt.Sprintf("scripting: compile %s: %v", origin, err))
	}

	return prog
}
