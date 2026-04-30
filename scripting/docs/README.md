# JavaScript Runtime

HyperTemplates scripts run in a small JavaScript environment designed for
template rendering and plugin-style value transformation.

This document describes what JavaScript code can use at runtime. It avoids
implementation details so script authors can reason in JavaScript terms.

## Runtime Model

Script source is treated as the body of a strict-mode function.

Use `return` to produce a value:

```js
return `Hello ${user.name}`
```

Statements are valid:

```js
const name = user.name.trim()
return name.toUpperCase()
```

If a script finishes without `return`, it produces no value.

## Inputs

Scripts receive input in one of two shapes.

When the input is an object, its top-level properties are available as local
constants:

```js
return `Hello ${user.name} from ${site}`
```

For example, this input:

```js
{
  user: { name: "Alice" },
  site: "HyperTemplates"
}
```

can be read as:

```js
return `${user.name} / ${site}`
```

Only valid strict-mode JavaScript binding names can become local constants.
Names such as `site`, `page`, `post_count`, and `theme2` are valid. Names such
as `page-title`, `2026`, or `site.name` are not valid local variable names.

When the input is any other value, it is available as `input`:

```js
return String(input).toUpperCase()
```

## Return Values

Scripts can return ordinary JavaScript values:

```js
return "hello"
return 42
return true
return null
return ["a", "b", "c"]
return { label: "Featured" }
```

Callers decide how to consume the returned value. Rendering-oriented callers
usually stringify the result, while lower-level callers may preserve arrays,
objects, numbers, booleans, null, or no value.

## Available JavaScript

The runtime provides standard JavaScript language features and built-ins used
for template logic, such as:

```js
Math.round(1.5)
String(input).trim()
Array.isArray(value)
Object.keys(data)
JSON.stringify(data)
```

Template literals are supported:

```js
return `${page.title} - ${site.name}`
```

## Top-Level APIs

The following names are available on `globalThis`:

```js
AggregateError
Array
ArrayBuffer
BigInt
BigInt64Array
BigUint64Array
Boolean
DataView
Date
Error
EvalError
Float32Array
Float64Array
Function
Infinity
Int16Array
Int32Array
Int8Array
JSON
Map
Math
NaN
Number
Object
Promise
Proxy
RangeError
ReferenceError
Reflect
RegExp
Set
String
Symbol
SyntaxError
TypeError
URIError
Uint16Array
Uint32Array
Uint8Array
Uint8ClampedArray
WeakMap
WeakSet
console
decodeURI
decodeURIComponent
encodeURI
encodeURIComponent
escape
eval
globalThis
isFinite
isNaN
parseFloat
parseInt
undefined
unescape
```

This is not a Node.js environment. Node.js APIs and browser APIs are not
available, including:

```js
require
import
process
Buffer
setTimeout
setInterval
fetch
window
document
localStorage
```

Scripts cannot load npm packages, read files, make network requests, or schedule
asynchronous work through timers.

## Console

The runtime provides `console` for diagnostic output:

```js
console.log("rendering", page.title)
console.info("using fallback title")
console.warn("missing optional image")
console.error("unexpected value", input)
console.debug("raw data", JSON.stringify(input))
```

Create a named logger with `console.logger(name)`:

```js
const logger = console.logger("plugin/title")

logger.log("rendering", input)
logger.warn("empty title")
```

Named loggers help separate output from different scripts or plugins.

## Frozen Globals

The runtime freezes high-impact global objects before scripts run:

```js
globalThis
Object
Object.prototype
Array
Array.prototype
Function
Function.prototype
console
```

This means scripts should not patch global objects or built-in prototypes:

```js
// Do not rely on this.
Array.prototype.first = function () {
  return this[0]
}
```

Keep helpers local to the script instead:

```js
function first(items) {
  return Array.isArray(items) ? items[0] : undefined
}

return first(input)
```

Freezing globals keeps one script from changing behavior for later scripts that
run in the same process.

## Time Limits

Scripts are expected to finish quickly. Long-running loops can be interrupted by
the caller:

```js
while (true) {
  // Avoid unbounded work.
}
```

Write scripts as small, deterministic transformations over their input.

## Patterns

Prefer explicit conversions at template boundaries:

```js
return String(input).trim()
```

Handle missing values directly:

```js
if (input == null) {
  return ""
}

return String(input)
```

Pass through values when a script is not responsible for a type:

```js
if (typeof input !== "string") {
  return input
}

return input.toUpperCase()
```

Avoid global state:

```js
// Prefer this.
const suffix = "!"
return `${input}${suffix}`
```

## Integration Reference

Go callers use `github.com/herdworks/hypertemplates-go/scripting` to compile
and run scripts.

The JavaScript-facing rules above apply to:

* `Eval`, `EvalString`, and `EvalStringStrict`
* `Script`, `ScriptContext`, and `ScriptTimeout`
* `Function`, `FunctionContext`, and `FunctionTimeout`
* Matching methods on `Engine`

`EvalString` applies JavaScript string coercion to the returned value and is the
best fit for template-literal rendering. `EvalStringStrict` requires the script
to return an actual JavaScript string.

`Script` compiles a reusable script body. `Function` compiles a JavaScript
plugin file; plugin-specific rules are documented in the plugins package.
