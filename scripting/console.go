package scripting

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/dop251/goja"
)

var consoleOutputMu sync.Mutex

func installConsole(vm *goja.Runtime) error {
	console := consoleValue(vm)
	if err := vm.Set("console", console); err != nil {
		return fmt.Errorf("set console: %w", err)
	}
	return nil
}

func consoleValue(vm *goja.Runtime) *goja.Object {
	console := loggerValue(vm, "scripting")
	_ = console.Set("logger", consoleLoggerMethod(vm))
	return console
}

func loggerValue(vm *goja.Runtime, name string) *goja.Object {
	console := vm.NewObject()
	_ = console.Set("name", name)
	_ = console.Set("log", consoleMethod(name, ""))
	_ = console.Set("info", consoleMethod(name, "INFO"))
	_ = console.Set("warn", consoleMethod(name, "WARN"))
	_ = console.Set("error", consoleMethod(name, "ERROR"))
	_ = console.Set("debug", consoleMethod(name, "DEBUG"))
	return console
}

func consoleLoggerMethod(vm *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		name := "scripting"
		if len(call.Arguments) > 0 {
			name = call.Arguments[0].String()
		}
		return loggerValue(vm, name)
	}
}

func consoleMethod(name, level string) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		message := consoleMessage(call.Arguments)

		consoleOutputMu.Lock()
		defer consoleOutputMu.Unlock()

		if level == "" {
			fmt.Fprintf(os.Stdout, "[%s] %s\n", name, message)
		} else {
			fmt.Fprintf(os.Stdout, "[%s] %s %s\n", name, level, message)
		}

		return goja.Undefined()
	}
}

func consoleMessage(args []goja.Value) string {
	if len(args) == 0 {
		return ""
	}

	parts := make([]string, 0, len(args))
	for _, arg := range args {
		parts = append(parts, arg.String())
	}
	return strings.Join(parts, " ")
}
