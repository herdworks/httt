package hypertemplates

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// ValidateRelativeFilePath validates configured relPath unless it is empty.
func ValidateRelativeFilePath(field string, relPath string) error {
	if relPath == "" {
		return nil
	}
	if _, err := CleanRelativeFilePath(relPath); err != nil {
		return fmt.Errorf("%s: %w", field, err)
	}
	return nil
}

// ResourceDirectoryName returns the configured resource directory name.
// When directory is empty, fallback is used without mutating the caller's config.
func ResourceDirectoryName(directory string, fallback string) string {
	if directory == "" {
		return fallback
	}
	return directory
}

// ListFiles returns sorted slash-separated file paths relative to root.
func ListFiles(root string, recursive bool) ([]string, error) {
	var paths []string
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve file root %q: %w", root, err)
	}

	info, err := os.Stat(rootAbs)
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("stat file root %q: %w", rootAbs, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("file root %q is not a directory", rootAbs)
	}

	if recursive {
		err = filepath.WalkDir(rootAbs, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			relPath, err := RelativeFilePath(rootAbs, path)
			if err != nil {
				return err
			}
			relPath = filepath.ToSlash(relPath)
			if _, err = CleanRelativeFilePath(relPath); err != nil {
				return err
			}
			paths = append(paths, relPath)
			return nil
		})
		if err != nil {
			return nil, err
		}
	} else {
		entries, err := os.ReadDir(rootAbs)
		if err != nil {
			return nil, fmt.Errorf("read file root %q: %w", rootAbs, err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			relPath, err := RelativeFilePath(rootAbs, filepath.Join(rootAbs, entry.Name()))
			if err != nil {
				return nil, err
			}
			relPath = filepath.ToSlash(relPath)
			if _, err = CleanRelativeFilePath(relPath); err != nil {
				return nil, err
			}
			paths = append(paths, relPath)
		}
	}

	slices.Sort(paths)
	return paths, nil
}

// ListFilesByPattern returns sorted slash-separated file paths relative to root
// that match include and do not match exclude.
func ListFilesByPattern(root string, recursive bool, include string, exclude string) ([]string, error) {
	regInclude, err := regexp.Compile(include)
	if err != nil {
		return nil, fmt.Errorf("compile include pattern %q: %w", include, err)
	}
	regExclude, err := regexp.Compile(exclude)
	if err != nil {
		return nil, fmt.Errorf("compile exclude pattern %q: %w", exclude, err)
	}

	files, err := ListFiles(root, recursive)
	if err != nil {
		return nil, err
	}

	paths := []string{}
	for _, path := range files {
		if include != "" && !regInclude.MatchString(path) {
			continue
		}
		if exclude != "" && regExclude.MatchString(path) {
			continue
		}
		paths = append(paths, path)
	}
	slices.Sort(paths)
	return paths, nil
}

// CleanRelativeFilePath validates relPath as a slash-separated relative file path.
func CleanRelativeFilePath(relPath string) (string, error) {
	if strings.Contains(relPath, "\\") {
		return "", fmt.Errorf("file path %q must use slash separators", relPath)
	}
	relPath = filepath.ToSlash(relPath)
	if relPath == "" || relPath == "." || !fs.ValidPath(relPath) {
		return "", fmt.Errorf("file path %q is not a valid relative path", relPath)
	}
	return relPath, nil
}

// LocalFilePath resolves a slash-separated relative file path under root.
func LocalFilePath(root string, relPath string) (string, error) {
	relPath, err := CleanRelativeFilePath(relPath)
	if err != nil {
		return "", err
	}
	localPath, err := filepath.Localize(relPath)
	if err != nil {
		return "", fmt.Errorf("localize file path %q: %w", relPath, err)
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve file root %q: %w", root, err)
	}
	return filepath.Join(rootAbs, localPath), nil
}

// LocalFilePaths resolves slash-separated relative file paths under root.
func LocalFilePaths(root string, relPaths []string) ([]string, error) {
	paths := make([]string, 0, len(relPaths))
	for _, relPath := range relPaths {
		path, err := LocalFilePath(root, relPath)
		if err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, nil
}

// RelativeFilePath returns file as a slash-separated path relative to root.
func RelativeFilePath(root string, file string) (string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve file root %q: %w", root, err)
	}
	fileAbs, err := filepath.Abs(file)
	if err != nil {
		return "", fmt.Errorf("resolve file path %q: %w", file, err)
	}
	relPath, err := filepath.Rel(rootAbs, fileAbs)
	if err != nil {
		return "", fmt.Errorf("resolve file path %q relative to %q: %w", fileAbs, rootAbs, err)
	}
	if relPath == "." || relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) || filepath.IsAbs(relPath) {
		return "", fmt.Errorf("file path %q escapes root %q", fileAbs, rootAbs)
	}
	relPath = filepath.ToSlash(relPath)
	if relPath, err = CleanRelativeFilePath(relPath); err != nil {
		return "", err
	}
	return relPath, nil
}
