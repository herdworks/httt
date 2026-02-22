# AGENTS.md

## Project-specific Instructions

<!-- Project-specific instructions take priority over general instructions -->

* Test: `go test ./... -v`
* Always run `go fmt ./...` before committing
* This library is consumed by `../libhypertexting` (C API for the app) and `../hyperctl` (CLI / static site generator)
* After committing changes, downstream repos pull updates via `go get github.com/herdworks/hypertemplates-go@<branch>`; for local testing, downstream repos can use `replace` directives in `go.mod`

<!-- See https://agents.md for more information -->
