# JavaScript Plugins

HyperTemplates plugins are JavaScript functions that transform values in
template pipelines.

This document is for plugin authors. It repeats the runtime constraints from
the scripting package where they affect plugin files.

## Plugin File Shape

A plugin file defines a top-level callable function and selects it with:

```js
const plugin = name
```

The selected identifier becomes the public plugin name.

```js
function upper(input) {
  if (input == null) {
    return input
  }

  return String(input).toUpperCase()
}

const plugin = upper
```

This registers a plugin named `upper`.

Function-valued constants are also valid:

```js
const title = (input) => {
  if (input == null) {
    return input
  }

  return String(input)
    .split(/\s+/)
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(" ")
}

const plugin = title
```

Comments and filenames do not name plugins. The selector is the contract:

```js
const plugin = title
```

## Using Plugins In Values

Plugins are used as downstream value transformers:

```gotemplate
${ page.title | upper }
```

That calls the selected JavaScript function with the upstream value:

```js
upper(page.title)
```

Plugins can return strings, numbers, booleans, arrays, objects, null, or the
original input. The final renderer decides how to display the result.

## Arguments

Plugins receive ordered arguments.

For a simple pipeline:

```gotemplate
${ page.title | upper }
```

the plugin receives one argument:

```js
upper(input)
```

For explicit arguments:

```gotemplate
${ page.updated_at | fmtdate "YYYY-MM-DD" }
```

the explicit arguments come first and the upstream pipeline value comes last:

```js
fmtdate("YYYY-MM-DD", input)
```

Write the JavaScript function in that order:

```js
function fmtdate(format = "", input = "") {
  if (input == null) {
    return input
  }

  return `${format}:${String(input)}`
}

const plugin = fmtdate
```

When a value expression uses a fallback:

```gotemplate
${ page.updated_at, page.created_at | fmtdate "YYYY-MM-DD" }
```

the pipeline resolves the input value first, then calls the plugin with the same
argument order:

```js
fmtdate("YYYY-MM-DD", resolvedInput)
```

## Input Policy

Each plugin owns its input policy.

Use pass-through behavior when the plugin only applies to some values:

```js
function upper(input) {
  if (typeof input !== "string") {
    return input
  }

  return input.toUpperCase()
}

const plugin = upper
```

Use explicit fallback behavior for missing values:

```js
function fallback(label = "", input = "") {
  if (input == null || input === "") {
    return label
  }

  return input
}

const plugin = fallback
```

Throw an error only when rendering should stop:

```js
function required(input) {
  if (input == null || input === "") {
    throw new Error("required value is missing")
  }

  return input
}

const plugin = required
```

Recoverable missing values should usually return `null`, an empty string, a
fallback, or the original input instead of throwing.

## Runtime Environment

Plugin files run in the same JavaScript runtime as lower-level scripts.

Available JavaScript includes standard language features and built-ins used for
value transformation:

```js
Math.round(1.5)
String(input).trim()
Array.isArray(value)
Object.keys(data)
JSON.stringify(data)
```

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

Plugins cannot load npm packages, read files, make network requests, or schedule
asynchronous work through timers.

## Console

Plugins can write diagnostic output with `console`:

```js
console.log("rendering", input)
console.info("using fallback")
console.warn("missing optional field")
console.error("unexpected value", input)
console.debug("raw value", JSON.stringify(input))
```

Use `console.logger(name)` to label output from a specific plugin:

```js
const logger = console.logger("plugin/upper")

function upper(input) {
  logger.log("rendering", input)
  return String(input).toUpperCase()
}

const plugin = upper
```

## Frozen Globals

The runtime freezes high-impact global objects before plugins run:

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

Do not patch globals or built-in prototypes:

```js
// Do not rely on this.
String.prototype.title = function () {
  return this.replace(/\b\w/g, (c) => c.toUpperCase())
}
```

Keep helpers inside the plugin file instead:

```js
function titleCase(value) {
  return String(value).replace(/\b\w/g, (c) => c.toUpperCase())
}

function title(input) {
  if (input == null) {
    return input
  }

  return titleCase(input)
}

const plugin = title
```

## Naming Rules

The plugin selector must be a top-level `const`:

```js
const plugin = upper
```

The selected name must refer to a top-level callable binding in the same file:

```js
function upper(input) {
  return String(input).toUpperCase()
}

const plugin = upper
```

These are not valid:

```js
// Missing selector.
function upper(input) {
  return String(input).toUpperCase()
}

// Selector is not const.
let plugin = upper

// Selector points at a value that is not a function.
const label = "upper"
const plugin = label

// Selector uses a reserved name.
function get(input) {
  return input
}
const plugin = get
```

`get` and `plugin` are reserved names.

If multiple plugin files resolve to the same name, the last loaded file wins.

## Authoring Checklist

* Select the public plugin with `const plugin = name`.
* Keep the selected function at the top level of the file.
* Put explicit pipeline arguments before the input value in the function
  signature.
* Return pass-through values for unsupported recoverable input types.
* Throw only when rendering should stop.
* Do not rely on Node.js, browser APIs, npm packages, timers, filesystem, or
  network access.
* Do not mutate globals or built-in prototypes.
