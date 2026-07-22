# scripting

`scripting` is a HyperTemplates subpackage for evaluating JavaScript scripts and plugin function files against caller-provided arguments.
It is designed for concurrent use, so one `Engine` can be shared safely across many goroutines.

## Goals

* Evaluate small JavaScript scripts against per-call inputs.
* Reuse compiled programs and pooled runtimes without leaking values between calls.
* Keep the API small, Go-native, and predictable.

## Installation

```sh
go get github.com/herdworks/hypertemplates-go
```

## Package Overview

The package import path is `github.com/herdworks/hypertemplates-go/scripting`, and the Go package name is `scripting`.
`Engine` is the concurrency-safe evaluator for advanced use cases.
`Eval` returns the exported Go value returned by a script.
`EvalString` applies JavaScript string coercion and is the best fit for template-style rendering.
`EvalStringStrict` requires the returned JavaScript value to already be a string.
`Script`, `ScriptContext`, and `ScriptTimeout` compile reusable script-body callables backed by a lazy package-default engine.
`Function`, `FunctionContext`, and `FunctionTimeout` compile JavaScript plugin files that declare `export default <function>`.
Matching `Engine` methods are available when you want explicit engine ownership.
`Options.EvalTimeout` applies an engine-level timeout to each evaluation.

Binding is intentionally binary:

* If the argument is `map[string]any`, its top-level keys become local constants inside the script when those keys are valid strict-mode JavaScript binding names.
* Every other argument shape is exposed as a single local constant named `input`.

`Eval` and `Script` source is wrapped in a strict-mode function body.
Use `return` to produce a value.
If a script completes without returning, `Eval` and compiled scripts return nil.
Plugin functions receive ordered arguments exactly as the Go caller supplies
them; maps are passed through as ordinary argument values.
The hardened runtime provides a `console` binding for standard-output logging.
`console.log` writes `[scripting] <message>`, while `console.info`,
`console.warn`, `console.error`, and `console.debug` write
`[scripting] <LEVEL> <message>`. Use `console.logger(name)` to create a
separate logger that writes `[<name>] <message>` and `[<name>] <LEVEL>
<message>` without mutating the shared runtime console.

The package uses [`goja`](https://github.com/dop251/goja), which provides standard JavaScript built-ins such as `Math`.
It does not provide Node.js globals such as `require`, `process`, `Buffer`, or `setTimeout`.
Each pooled runtime is hardened by freezing `globalThis`, `Object`, `Object.prototype`, `Array`, `Array.prototype`, `Function`, and `Function.prototype` so broad cross-evaluation monkey-patching fails fast.

## Basic Example

```go
package main

import (
	"context"
	"fmt"

	"github.com/herdworks/hypertemplates-go/scripting"
)

func main() {
	engine := scripting.NewEngine(scripting.Options{})
	defer engine.Close()

	got, err := engine.EvalString(
		context.Background(),
		"return `Hello ${user.name} from ${site}`",
		map[string]any{
			"user": map[string]any{"name": "Alice"},
			"site": "HyperTemplates",
		},
	)
	if err != nil {
		panic(err)
	}

	fmt.Println(got)
}
```

## Compiled Script Example

```go
package main

import (
	"fmt"

	"github.com/herdworks/hypertemplates-go/scripting"
)

func main() {
	formatGreeting, err := scripting.Script(
		"return `Hello ${user.name}`",
		map[string]any{"user": nil},
	)
	if err != nil {
		panic(err)
	}

	got, err := formatGreeting(map[string]any{
		"user": map[string]any{"name": "Alice"},
		"site": "ignored because it was not in the sample shape",
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(got)
}
```

## Compile JavaScript Files

`Function` accepts one JavaScript plugin file. Plugin files use ES module
syntax — they default-export a function:

```js
// scripts/upper.js
export default function(input) {
	if (typeof input !== "string") {
		return input
	}

	return input.toUpperCase()
}
```

The plugin name is derived from the filename (sans extension), so
`scripts/upper.js` registers an `upper` plugin usable as
`${ upper(page.title) }`. To compile a batch of plugin files for use with
the `variable` package, call `variable.LoadPlugins`:

```go
package main

import (
	htvariable "github.com/herdworks/hypertemplates-go/variable"
)

func main() {
	plugins, err := htvariable.LoadPlugins([]string{"scripts/upper.js", "scripts/title.js"})
	if err != nil {
		panic(err)
	}

	// Use plugins with Document.Plugins, or pass plugins.FuncMap() to variable.RenderString / variable.RenderNodes.
	_ = plugins
}
```

If multiple files resolve to the same plugin name, the last file wins.
Use `variable.MergePlugins` to combine plugin groups; later groups take
precedence.

The runtime is `goja`. There is no Node, no npm, no `fs`, no `fetch`, and
no module system: `import` declarations are rejected at load time and
`require()` calls throw `*PluginError` at runtime. The features you have
access to are the standard ECMAScript built-ins (`Math`, `JSON`, `Object`,
etc.), `console`, and whatever you compose from there.

Plugin functions can write diagnostic output with `console`:

```js
const logger = console.logger("plugin/upper")

export default function(input) {
	logger.log("rendering", input)
	return String(input).toUpperCase()
}
```

```go
rendered, err := htvariable.RenderString(
	`${ upper(page.title) }`,
	data,
	htvariable.SyntaxDefault,
	plugins.FuncMap(),
)
if err != nil {
	panic(err)
}
```

Plugin files can use any function shape that's a valid expression — arrow
functions, async functions, named or anonymous:

```js
// scripts/title.js
export default (input) => {
	if (input == null) {
		return input
	}

	return String(input)
		.split(/\s+/)
		.map(word => word.charAt(0).toUpperCase() + word.slice(1))
		.join(" ")
}
```

Plugins receive resolved keypaths and literal arguments in source order, so
`${ fmtdate(page.updated_at, page.created_at, "YYYY-MM-DD") }` calls JavaScript
as `plugin(updatedAt, createdAt, "YYYY-MM-DD")`. Missing keypaths are passed as
nil values and the plugin owns any fallback policy.

If you need to install the plugins on a full HyperTemplates template, assign the map directly:

```go
template.Plugins = plugins
```

## Concurrent Usage

One `Engine` can be shared across goroutines.
Each call receives its own arguments, and compiled programs are cached by script source and binding shape.
All non-map arguments share one `input` cache shape.
Map arguments are cached by their sorted top-level keys.
Plugin functions are cached by plugin source.

```go
package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/herdworks/hypertemplates-go/scripting"
)

func main() {
	engine := scripting.NewEngine(scripting.Options{})
	defer engine.Close()

	names := []string{"Alice", "Bob", "Carol"}

	var wg sync.WaitGroup
	for _, name := range names {
		name := name
		wg.Add(1)

		go func() {
			defer wg.Done()

			got, err := engine.EvalString(
				context.Background(),
				"return `hi ${user.name}`",
				map[string]any{"user": map[string]any{"name": name}},
			)
			if err != nil {
				panic(err)
			}

			fmt.Println(got)
		}()
	}

	wg.Wait()
}
```

## Timeouts And Cancellation

`Eval` respects both the caller's `context.Context` and the optional engine-level timeout.
`ScriptContext` keeps that same caller-controlled cancellation model for compiled script callables.
`ScriptTimeout` applies a fixed timeout to each script invocation.
`Script` is a convenience wrapper over `ScriptTimeout` that uses `250 * time.Millisecond`.
Use `ScriptContext` when you need per-call deadlines or cancellation for script bodies.
The plugin-oriented `FunctionContext` and `FunctionTimeout` variants provide
the same cancellation and timeout controls for plugin functions.

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/herdworks/hypertemplates-go/scripting"
)

func main() {
	fn, err := scripting.ScriptContext("while (true) {}", nil)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = fn(ctx, nil)
	if errors.Is(err, scripting.ErrInterrupted) {
		fmt.Println("script interrupted")
		return
	}
	if err != nil {
		panic(err)
	}
}
```

## Troubleshooting

* If a map key is not available as a bare identifier, make sure the argument is exactly `map[string]any`.
* If evaluation fails before running because of a top-level map key, rename that key to a valid strict-mode JavaScript binding name such as `user_name` or nest it under a valid parent key.
* If you pass a struct, slice, scalar, or any other non-map value, access it through `input`.
* If a compiled script rejects an argument, check whether the runtime argument still matches the sample shape you used at compile time.
* If a plugin file fails to compile, confirm it has exactly one `export default <expression>` and that the expression evaluates to a function. CJS-style `module.exports = …` is **not** supported — esbuild rejects it in IIFE format.
* If a plugin fails at load time with `PluginError: import is not supported in plugins yet`, remove the `import` declaration. There is no module system; helpers must live in the same file.
* If a plugin fails at runtime with `PluginError: require is not supported in plugins yet`, the wrapper's `require` stub was invoked. This usually means esbuild transformed an `import` (which the pre-scan should normally catch) or the source called `require()` directly.
* If a script gets a `TypeError` while assigning to `globalThis` or a core prototype, runtime hardening blocked a persistent side effect on the pooled VM.
* If `Script` or `Function` interrupts unexpectedly, switch to the matching `Context` or `Timeout` variant so the timeout policy is explicit.

## Development

Run the test suite with:

```sh
go test ./...
```
