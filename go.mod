// hypertemplates-go module
//
// reference: https://go.dev/doc/modules/gomod-ref#replace
module github.com/herdworks/hypertemplates-go

go 1.23.5

// first-party dependencies
require github.com/herdworks/hyperfeed-go v0.18.1-0.20260311165907-b49847b0e80c

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
	github.com/dlclark/regexp2 v1.7.0 // indirect
	gopkg.in/yaml.v2 v2.3.0 // indirect
)
