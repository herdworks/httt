package theme

import (
	"embed"
	"errors"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
)

// Builtin describes a slot in the layout where embedded HTML is injected
// subject to theme override or empty-file opt-out.
type Builtin struct {
	Name     string
	Selector string
	Mode     string
	Fragment string
}

//go:embed embedded
var embeddedFS embed.FS

// BuiltinsFS is the filesystem consulted when a theme resource is not found
// on disk. Defaults to the fragments embedded with this package, exposed at
// keys rooted at "builtins/...". Caller programs may replace it with any fs.FS
// whose keys begin at "builtins/" to ship their own built-in resource pack.
var BuiltinsFS fs.FS = MustSubFS(embeddedFS, "embedded")

var defaultBuiltins = []Builtin{
	{Name: "head_start", Selector: "head", Mode: "prepend", Fragment: "fragments/builtin/head_start"},
}

// DefaultBuiltins returns the immutable default builtin slot registry.
func DefaultBuiltins() []Builtin {
	return slices.Clone(defaultBuiltins)
}

// ReadBuiltinResource returns embedded bytes for a theme-relative ref.
// It returns (nil, false, nil) when BuiltinsFS has no entry at that path.
func ReadBuiltinResource(ref string) ([]byte, bool, error) {
	embedPath := path.Join("builtins", filepath.ToSlash(ref))
	if filepath.Ext(embedPath) == "" {
		embedPath += ".html"
	}
	data, err := fs.ReadFile(BuiltinsFS, embedPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

// MustSubFS returns an fs.FS rooted at dir within fsys, panicking on error.
func MustSubFS(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}
