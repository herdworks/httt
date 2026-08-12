package migrate

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	htfiles "github.com/herdworks/httt/files"
	httheme "github.com/herdworks/httt/theme"
)

func (migration *Migration) prepareOutput(theme httheme.Theme) (httheme.Theme, error) {
	if migration.OutputDir == "" {
		return theme, nil
	}
	sourceRoot := theme.RootDir()
	outputRoot, err := filepath.Abs(migration.OutputDir)
	if err != nil {
		return httheme.Theme{}, fmt.Errorf("resolve output directory %q: %w", migration.OutputDir, err)
	}
	if pathsOverlap(sourceRoot, outputRoot) {
		return httheme.Theme{}, fmt.Errorf("output directory %q must be disjoint from theme root %q", outputRoot, sourceRoot)
	}
	if err := requireEmptyDirectory(outputRoot); err != nil {
		return httheme.Theme{}, err
	}
	if err := os.MkdirAll(outputRoot, 0o755); err != nil {
		return httheme.Theme{}, fmt.Errorf("create output directory %q: %w", outputRoot, err)
	}
	paths, err := theme.FilePaths()
	if err != nil {
		return httheme.Theme{}, fmt.Errorf("list theme files: %w", err)
	}
	for _, relative := range paths {
		source, err := htfiles.LocalPath(sourceRoot, relative)
		if err != nil {
			return httheme.Theme{}, fmt.Errorf("resolve source theme file %q: %w", relative, err)
		}
		target, err := htfiles.LocalPath(outputRoot, relative)
		if err != nil {
			return httheme.Theme{}, fmt.Errorf("resolve output theme file %q: %w", relative, err)
		}
		if err := copyRegularFile(source, target); err != nil {
			return httheme.Theme{}, fmt.Errorf("copy theme file %q: %w", relative, err)
		}
	}
	manifest, err := htfiles.RelativePath(sourceRoot, theme.URI.Path)
	if err != nil {
		return httheme.Theme{}, fmt.Errorf("resolve theme configuration: %w", err)
	}
	outputThemePath, err := htfiles.LocalPath(outputRoot, manifest)
	if err != nil {
		return httheme.Theme{}, fmt.Errorf("resolve output theme configuration: %w", err)
	}
	outputTheme, err := loadTheme(outputThemePath)
	if err != nil {
		return httheme.Theme{}, fmt.Errorf("load output theme: %w", err)
	}
	return outputTheme, nil
}

func requireEmptyDirectory(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat output directory %q: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("output directory %q is a symlink", path)
	}
	if !info.IsDir() {
		return fmt.Errorf("output path %q is not a directory", path)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("read output directory %q: %w", path, err)
	}
	if len(entries) != 0 {
		return fmt.Errorf("output directory %q is not empty", path)
	}
	return nil
}

func pathsOverlap(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	return pathWithin(left, right) || pathWithin(right, left)
}

func pathWithin(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	if err != nil || filepath.IsAbs(relative) {
		return false
	}
	return relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func copyRegularFile(source, target string) error {
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("source %q is not a regular file", source)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
