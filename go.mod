// hypertemplates-go module
//
// reference: https://go.dev/doc/modules/gomod-ref#replace
module github.com/herdworks/hypertemplates-go

go 1.23.5

// first-party dependencies
require github.com/herdworks/hyperfeed-go v0.19.1

// direct dependencies
require (
	github.com/alecthomas/chroma/v2 v2.2.0
	github.com/andybalholm/cascadia v1.3.2
	github.com/yuin/goldmark v1.7.8
	github.com/yuin/goldmark-emoji v1.0.4
	github.com/yuin/goldmark-highlighting/v2 v2.0.0-20230729083705-37449abec8cc
	github.com/yuin/goldmark-meta v1.1.0
	golang.org/x/net v0.43.0
	gopkg.in/yaml.v3 v3.0.1
)

// local development dependencies (replacements)
//
// uncomment to use local dependencies
// replace github.com/herdworks/hyperfeed-go => ../hyperfeed-go

// transitive dependencies
require (
	github.com/dlclark/regexp2 v1.11.4 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)

require (
	github.com/dop251/goja v0.0.0-20260311135729-065cd970411c
	github.com/evanw/esbuild v0.28.0
)

require (
	github.com/go-sourcemap/sourcemap v2.1.3+incompatible // indirect
	github.com/google/pprof v0.0.0-20230207041349-798e818bf904 // indirect
	golang.org/x/sys v0.35.0 // indirect
	golang.org/x/text v0.28.0 // indirect
)
