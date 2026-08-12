package types

import (
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"

	httheme "github.com/herdworks/httt/theme"
)

// File identifies one migratable theme resource.
type File struct {
	Path string
	Kind FileKind
}

// FileKind identifies whether a theme HTML file is parsed as a layout or fragment.
type FileKind uint8

// Files groups theme resources into their migration processing order.
type Files struct {
	Layouts   []File
	Fragments []File
}

const (
	LayoutFile FileKind = iota
	FragmentFile
)

// Ordered returns layouts followed by fragments.
func (files Files) Ordered() []File {
	ordered := make([]File, 0, len(files.Layouts)+len(files.Fragments))
	ordered = append(ordered, files.Layouts...)
	ordered = append(ordered, files.Fragments...)
	return ordered
}

// DiscoverFiles returns sorted migratable HTML files from a validated theme.
func DiscoverFiles(theme httheme.Theme) (Files, error) {
	paths, err := theme.FilePaths()
	if err != nil {
		return Files{}, fmt.Errorf("list theme files: %w", err)
	}
	layoutDirectory := path.Clean(filepath.ToSlash(theme.LayoutsDirName()))
	blockDirectory := path.Clean(filepath.ToSlash(theme.BlocksDirName()))
	fragmentDirectory := path.Clean(filepath.ToSlash(theme.FragmentsDirName()))
	files := Files{}
	for _, file := range paths {
		file = path.Clean(filepath.ToSlash(file))
		if path.Ext(file) != ".html" {
			continue
		}
		var kind FileKind
		switch {
		case pathInDirectory(file, layoutDirectory):
			kind = LayoutFile
		case pathInDirectory(file, blockDirectory), pathInDirectory(file, fragmentDirectory):
			kind = FragmentFile
		default:
			continue
		}
		resource := File{Path: file, Kind: kind}
		if kind == LayoutFile {
			files.Layouts = append(files.Layouts, resource)
		} else {
			files.Fragments = append(files.Fragments, resource)
		}
	}
	sort.Slice(files.Layouts, func(left, right int) bool {
		return files.Layouts[left].Path < files.Layouts[right].Path
	})
	sort.Slice(files.Fragments, func(left, right int) bool {
		return files.Fragments[left].Path < files.Fragments[right].Path
	})
	return files, nil
}

func pathInDirectory(file, directory string) bool {
	return file != directory && strings.HasPrefix(file, directory+"/")
}
