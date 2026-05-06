package hypertemplates

import (
	"bytes"
	"embed"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"golang.org/x/net/html"
)

//go:embed embedded
var embeddedFS embed.FS

// BuiltinsFS is the filesystem consulted when a theme resource is not found
// on disk. Defaults to the fragments embedded with this package, exposed at
// keys rooted at "builtins/...". Caller programs (e.g. hypertexting-go) may
// replace it with any fs.FS whose keys begin at "builtins/" to ship their
// own built-in resource pack.
var BuiltinsFS fs.FS = MustSubFS(embeddedFS, "embedded")

// MustSubFS returns an fs.FS rooted at dir within fsys, panicking on error.
// Intended for caller programs that ship their own embedded resource pack
// alongside an embed.FS whose keys are prefixed by the embed-target name:
//
//	//go:embed embedded
//	var embeddedFS embed.FS
//	hypertemplates.BuiltinsFS = hypertemplates.MustSubFS(embeddedFS, "embedded")
//
// fs.Sub cannot fail when dir is a compile-time constant matching the
// //go:embed target, so the panic is reserved for that unreachable case.
func MustSubFS(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}

// Builtin describes a slot in the layout where embedded HTML is injected
// (subject to theme override or empty-file opt-out).
type Builtin struct {
	Name     string // diagnostic, e.g. "head_start"
	Selector string // cascadia, e.g. "head"
	Mode     string // "prepend" | "append" | "before" | "after"
	Fragment string // resource ref, e.g. "fragments/builtin/head_start"
}

// DefaultBuiltins is the registry of slots applied by Template.RenderBuiltins.
// Callers may append to or replace entries to register additional builtins.
var DefaultBuiltins = []Builtin{
	{Name: "head_start", Selector: "head", Mode: "prepend", Fragment: "fragments/builtin/head_start"},
}

// getBuiltinResource returns embedded bytes for a theme-relative ref.
// Returns (nil, false, nil) when BuiltinsFS has no entry at that path.
func getBuiltinResource(ref string) ([]byte, bool, error) {
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

// RenderBuiltins iterates DefaultBuiltins and injects each slot's fragment
// into the layout. Theme files at the same relative path override the
// embedded defaults; an empty theme file disables the slot. Skipped when the
// template is in BlockMode (block partials don't get builtins applied).
func (template *Template) RenderBuiltins() error {
	if template.BlockMode {
		return nil
	}
	for _, slot := range DefaultBuiltins {
		if err := template.renderBuiltinSlot(slot); err != nil {
			return err
		}
	}
	return nil
}

func (template *Template) renderBuiltinSlot(slot Builtin) error {
	fragmentBytes, found, err := template.resolveBuiltinFragment(slot.Fragment)
	if err != nil {
		return err
	}
	if !found {
		return nil // no theme file, no embedded default — silently skip
	}
	if len(fragmentBytes) == 0 {
		return nil // empty theme override — opt-out
	}

	target := Query(template.Layout, slot.Selector)
	if target == nil {
		return nil // no selector match — silently skip
	}

	parseParent := target
	switch slot.Mode {
	case "before", "after":
		if target.Parent == nil {
			return nil // can't insert sibling without a parent
		}
		parseParent = target.Parent
	}

	fragments, err := html.ParseFragment(bytes.NewReader(fragmentBytes), parseParent)
	if err != nil {
		return err
	}

	switch slot.Mode {
	case "prepend":
		anchor := target.FirstChild
		for _, node := range fragments {
			if anchor == nil {
				target.AppendChild(node)
			} else {
				target.InsertBefore(node, anchor)
			}
		}
	case "append":
		for _, node := range fragments {
			target.AppendChild(node)
		}
	case "before":
		for _, node := range fragments {
			target.Parent.InsertBefore(node, target)
		}
	case "after":
		anchor := target.NextSibling
		for _, node := range fragments {
			if anchor == nil {
				target.Parent.AppendChild(node)
			} else {
				target.Parent.InsertBefore(node, anchor)
			}
		}
	}
	return nil
}

// resolveBuiltinFragment tries the theme path first and falls back to
// BuiltinsFS. Returns (bytes, found, err). An existing-but-empty theme file
// is reported as found with zero-length bytes (signals opt-out to callers).
func (template *Template) resolveBuiltinFragment(ref string) ([]byte, bool, error) {
	if template.Theme != nil {
		themePath := template.Theme.ResourcePath(ref)
		data, err := os.ReadFile(themePath)
		if err == nil {
			return data, true, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, false, err
		}
	}
	return getBuiltinResource(ref)
}
