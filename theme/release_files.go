package theme

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// OpenRoot anchors output operations to the static tree, even if an ancestor
// changes during a release. Reject a symlink used as the static root itself.
func openReleaseRoot(path string) (*os.Root, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("release static root %q is a symlink", path)
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return nil, err
	}
	info, err = os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("release static root %q is a symlink", path)
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	anchored, err := root.Stat(".")
	if err != nil || !os.SameFile(info, anchored) {
		root.Close()
		return nil, fmt.Errorf("release static root changed while opening %q", path)
	}
	return root, nil
}

// Reject symlink destinations and ancestors inside the output tree, including
// links to other locations inside the tree. Root confines the actual I/O too.
func rejectReleaseSymlinks(root *os.Root, name string) error {
	if !filepath.IsLocal(name) {
		return fmt.Errorf("release output %q must stay under the static root", name)
	}
	current := "."
	parts := strings.Split(filepath.Clean(name), string(filepath.Separator))
	for i, part := range parts {
		current = filepath.Join(current, part)
		info, err := root.Lstat(current)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("release output path %q contains a symlink at %q", name, current)
		}
		if i < len(parts)-1 && !info.IsDir() {
			return fmt.Errorf("release output ancestor %q is not a directory", current)
		}
	}
	return nil
}

// Native packaging writes only into its private scratch directory. Copy the
// finished bytes into a root-confined stage before installing the archive.
func copyReleaseArchive(root *os.Root, name, source string) error {
	if err := rejectReleaseSymlinks(root, name); err != nil {
		return err
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	if copyErr == nil {
		copyErr = output.Sync()
	}
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func releaseArchiveDigest(root *os.Root, name string) (string, error) {
	if err := rejectReleaseSymlinks(root, name); err != nil {
		return "", err
	}
	file, err := root.Open(name)
	if err != nil {
		return "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("archive %q is not a regular file", name)
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func sameReleaseArchivePath(indexed, candidate string) bool {
	ref, err := url.Parse(indexed)
	if err != nil || ref.Scheme != "" || ref.Host != "" || ref.RawQuery != "" || ref.ForceQuery || ref.Fragment != "" {
		return false
	}
	// Only origin-relative locations can name a local output. Preserve query,
	// host, and encoded-path distinctions instead of guessing how a server maps
	// them. Dot segments and index-relative/root-relative spellings normalize.
	base := &url.URL{Path: ReleaseIndexPath}
	return base.ResolveReference(ref).String() == candidate
}
