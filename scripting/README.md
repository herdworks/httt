# scripting

`scripting` is a HyperTemplates subpackage for evaluating JavaScript expressions against caller-provided arguments.
It is designed for concurrent use, so one `Engine` can be shared safely across many goroutines.

## Goals

* Evaluate template literals and small JavaScript expressions against per-call inputs.
* Reuse compiled programs and pooled runtimes without leaking values between calls.
* Keep the API small, Go-native, and predictable.

## Installation

```sh
go get github.com/herdworks/hypertemplates-go
```

## Package Overview

The package import path is `github.com/herdworks/hypertemplates-go/scripting`, and the Go package name is `scripting`.
`Engine` is the concurrency-safe evaluator for advanced use cases.
`Eval` returns the exported Go value for an expression.
`EvalString` applies JavaScript string coercion and is the best fit for template-style rendering.
`EvalStringStrict` requires the final JavaScript value to already be a string.
`Function`, `FunctionContext`, and `FunctionTimeout` compile reusable Go callables backed by a lazy package-default engine.
Matching `Engine` methods are available when you want explicit engine ownership.
`Options.EvalTimeout` applies an engine-level timeout to each evaluation.

Binding is intentionally binary:

* If the argument is `map[string]any`, its top-level keys become local constants inside the expression when those keys are valid strict-mode JavaScript binding names.
* Every other argument shape is exposed as a single local constant named `input`.

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
		"`Hello ${user.name} from ${site}`",
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

## Compiled Function Example

```go
package main

import (
	"fmt"

	"github.com/herdworks/hypertemplates-go/scripting"
)

func main() {
	formatGreeting, err := scripting.Function(
		"`Hello ${user.name}`",
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

`Function` accepts one JavaScript expression.
To compile one reusable Go callable per JavaScript file, read each file and pass its contents to `scripting.Function` independently.

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/herdworks/hypertemplates-go/scripting"
	htvalue "github.com/herdworks/hypertemplates-go/value"
)

func pluginsFromFiles(paths []string) (plugins htvalue.Plugins, err error) {
	plugins = make(htvalue.Plugins, len(paths))

	for _, path := range paths {
		var name, source string
		var ok bool
		var fn func(any) (any, error)
		if src, err := os.ReadFile(path); err == nil {
			source = string(src)
		} else {
			return nil, fmt.Errorf("read script %q: %w", path, err)
		}

		if name, ok, err = scripting.PluginName(source); err != nil {
			return nil, fmt.Errorf("parse plugin name %q: %w", path, err)
		} else if !ok {
			basename := filepath.Base(path)
			name = strings.TrimSuffix(basename, filepath.Ext(basename))
		}
		if name == "httd" || name == "default" {
			return nil, fmt.Errorf("reserved plugin name %q from %q", name, path)
		}
		if _, exists := plugins[name]; exists {
			return nil, fmt.Errorf("duplicate plugin name %q from %q", name, path)
		}
		if fn, err = scripting.Function(source, nil); err != nil {
			return nil, fmt.Errorf("compile script %q: %w", path, err)
		}
		plugins[name] = fn
	}

	return plugins, nil
}

func main() {
	plugins, err := pluginsFromFiles([]string{"scripts/upper.js", "scripts/title.js"})
	if err != nil {
		panic(err)
	}

	// Use plugins with Template.Plugins, or pass plugins.FuncMap() to value.Render.
	_ = plugins
}
```

Each leading `ht:plugin` comment becomes the value pipeline function name.
If a file does not declare a name, the helper falls back to the filename stem.
For example, `scripts/upper.js` registers an `upper` plugin usable as `${ page.title | upper }`.
Because pipeline input is passed as the function argument, JavaScript file plugins usually read from `input`:

```js
// ht:plugin upper
input.toUpperCase()
```

```go
rendered, err := htvalue.Render(
	`${ page.title | upper }`,
	data,
	htvalue.SyntaxDefault,
	plugins.FuncMap(),
)
if err != nil {
	panic(err)
}
```

If a JavaScript file contains statements, make the file an expression by wrapping those statements in an IIFE:

```js
(() => {
	if (input == null) {
		return input
	}

	return String(input).toUpperCase()
})()
```

If you need to install the plugins on a full HyperTemplates template, assign the map directly:

```go
template.Plugins = plugins
```

## Concurrent Usage

One `Engine` can be shared across goroutines.
Each call receives its own argument, and compiled programs are cached by expression and binding shape.
All non-map arguments share one `input` cache shape.
Map arguments are cached by their sorted top-level keys.

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
				"`hi ${user.name}`",
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
`FunctionContext` keeps that same caller-controlled cancellation model for compiled callables.
`FunctionTimeout` applies a fixed timeout to each invocation.
`Function` is a convenience wrapper over `FunctionTimeout` that uses `250 * time.Millisecond`.
Use `FunctionContext` when you need per-call deadlines or cancellation.

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
	fn, err := scripting.FunctionContext("(()=>{while(true){}})()", nil)
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
* If a compiled function rejects an argument, check whether the runtime argument still matches the sample shape you used at compile time.
* If a script gets a `TypeError` while assigning to `globalThis` or a core prototype, runtime hardening blocked a persistent side effect on the pooled VM.
* If `Function` interrupts unexpectedly, switch to `FunctionContext` or `FunctionTimeout` so the timeout policy is explicit.

## Development

Run the test suite with:

```sh
go test ./...
```
