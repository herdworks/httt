package theme

import (
	"archive/zip"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/herdworks/httt/files"
)

var releaseSlugSeparators = regexp.MustCompile(`[^a-z0-9]+`)
var releaseVersion = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.+_-]*$`)

// Release prepares a local archive and the site's canonical single-theme index.
// staticDir is the website static directory. archivePath is slash-separated and
// relative to staticDir; empty uses .hypertexting/themes/<slug>-v<version>.zip.
// Metadata is read from theme.json, never from caller-provided display defaults.
// Archives are staged, read back, and installed without overwriting existing
// bytes before an atomic index replacement. A failed index write can leave a
// valid unindexed archive; retrying with the same bytes safely completes it.
func (theme *Theme) Release(staticDir, archivePath string) (Release, error) {
	if theme.URI == nil || filepath.Base(theme.URI.Path) != "theme.json" {
		return Release{}, fmt.Errorf("release requires a selected theme.json")
	}
	content, err := os.ReadFile(theme.URI.Path)
	if err != nil {
		return Release{}, err
	}
	source := Theme{URI: theme.URI}
	if err := json.Unmarshal(content, &source); err != nil {
		return Release{}, err
	}
	if source.Name == "" || strings.ContainsAny(source.Name, `/\\`) || source.Name == "." || source.Name == ".." {
		return Release{}, fmt.Errorf("release requires a valid theme name in theme.json")
	}
	if !releaseVersion.MatchString(source.Version) {
		return Release{}, fmt.Errorf("release requires a valid version in theme.json")
	}
	if err := source.Validate(); err != nil {
		return Release{}, err
	}
	slug := strings.Trim(releaseSlugSeparators.ReplaceAllString(strings.ToLower(source.Name), "-"), "-")
	if slug == "" {
		return Release{}, fmt.Errorf("theme name must contain an ASCII letter or number for its release slug")
	}
	if archivePath == "" {
		archivePath = ".hypertexting/themes/" + slug + "-v" + source.Version + ".zip"
	}
	archive, err := files.LocalPath(staticDir, archivePath)
	if err != nil {
		return Release{}, err
	}
	indexPath, err := files.LocalPath(staticDir, strings.TrimPrefix(ReleaseIndexPath, "/"))
	if err != nil {
		return Release{}, err
	}
	indexDir := filepath.Dir(indexPath)
	lockPath := filepath.Join(indexDir, ".theme-release.lock")
	if archive == indexDir || archive == indexPath || archive == lockPath {
		return Release{}, fmt.Errorf("archive path conflicts with release metadata")
	}
	outputs, err := openReleaseRoot(staticDir)
	if err != nil {
		return Release{}, err
	}
	defer outputs.Close()
	indexName := filepath.FromSlash(strings.TrimPrefix(ReleaseIndexPath, "/"))
	indexDirName := filepath.Dir(indexName)
	lockName := filepath.Join(indexDirName, ".theme-release.lock")
	archiveName := filepath.FromSlash(archivePath)
	for _, name := range []string{indexName, lockName, archiveName} {
		if err := rejectReleaseSymlinks(outputs, name); err != nil {
			return Release{}, err
		}
	}
	if err := outputs.MkdirAll(indexDirName, 0o755); err != nil {
		return Release{}, err
	}
	lock, err := outputs.OpenFile(lockName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return Release{}, fmt.Errorf("lock theme release (another release may be running): %w", err)
	}
	defer outputs.Remove(lockName)
	if err := lock.Close(); err != nil {
		return Release{}, err
	}

	// Raw messages preserve unrelated metadata and previous version records.
	document := map[string]json.RawMessage{}
	versions := map[string]json.RawMessage{}
	var existing ReleaseIndex
	previous, err := outputs.ReadFile(indexName)
	if err == nil {
		if err := json.Unmarshal(previous, &existing); err != nil {
			return Release{}, fmt.Errorf("parse existing theme index: %w", err)
		}
		if err := existing.Validate(); err != nil {
			return Release{}, err
		}
		if existing.Name != source.Name {
			return Release{}, fmt.Errorf("site already releases theme %q, not %q", existing.Name, source.Name)
		}
		_ = json.Unmarshal(previous, &document)
		_ = json.Unmarshal(document["versions"], &versions)
	} else if !os.IsNotExist(err) {
		return Release{}, err
	}

	archiveURL := (&url.URL{Path: "/" + archivePath}).String()
	if prior, exists := existing.Versions[source.Version]; exists {
		if !sameReleaseArchivePath(prior.URL, archiveURL) {
			return Release{}, fmt.Errorf("theme version %q already has a different archive path", source.Version)
		}
		// Keep an indexed URL stable, including equivalent relative spellings.
		archiveURL = prior.URL
	}
	stageName := filepath.Join(indexDirName, ".theme-release-"+rand.Text())
	if err := outputs.Mkdir(stageName, 0o700); err != nil {
		return Release{}, err
	}
	defer outputs.RemoveAll(stageName)
	// Native Archive takes a filesystem path. Give it a private scratch
	// directory outside the output tree so output symlinks cannot redirect it.
	stage, err := os.MkdirTemp("", "httt-theme-release-*")
	if err != nil {
		return Release{}, err
	}
	defer os.RemoveAll(stage)
	staged := filepath.Join(stage, "archive.zip")
	// Omit only exact release files and this invocation's staging directories.
	// Other assets under .hypertexting remain native theme resources.
	source.archiveExcludes = []string{indexPath, lockPath, archive}
	source.archiveStageDirs = []string{filepath.Join(outputs.Name(), stageName), stage}

	for _, prior := range existing.Versions {
		ref, _ := url.Parse(prior.URL) // existing.Validate checked this URL
		if ref.Scheme == "" && ref.Host == "" {
			// Relative archive URLs are relative to the canonical index, just
			// as they are for clients. External URLs imply no local file.
			base := &url.URL{Path: ReleaseIndexPath}
			relative := strings.TrimPrefix(base.ResolveReference(ref).Path, "/")
			if relative == "" {
				continue
			}
			local, err := files.LocalPath(staticDir, relative)
			if err != nil {
				return Release{}, err
			}
			// Only files at prior local URL paths are publication artifacts.
			// A URL served by a directory or handler must not exclude that
			// entire directory of genuine theme resources.
			if err := rejectReleaseSymlinks(outputs, filepath.FromSlash(relative)); err != nil {
				return Release{}, err
			}
			if info, err := outputs.Lstat(filepath.FromSlash(relative)); err == nil && info.Mode().IsRegular() {
				source.archiveExcludes = append(source.archiveExcludes, local)
			} else if err != nil && !os.IsNotExist(err) {
				return Release{}, err
			}
		}
	}
	if err := source.Archive(staged, io.Discard); err != nil {
		return Release{}, err
	}
	if err := verifyReleaseArchive(staged, source.Name, source.Version); err != nil {
		return Release{}, err
	}
	digest, err := archiveDigest(staged)
	if err != nil {
		return Release{}, err
	}
	if prior, exists := existing.Versions[source.Version]; exists && !strings.EqualFold(prior.SHA256, digest) {
		return Release{}, fmt.Errorf("theme version %q already has different archive bytes", source.Version)
	}
	if err := rejectReleaseSymlinks(outputs, archiveName); err != nil {
		return Release{}, err
	}
	if err := outputs.MkdirAll(filepath.Dir(archiveName), 0o755); err != nil {
		return Release{}, err
	}
	stagedName := filepath.Join(stageName, "archive.zip")
	if err := copyReleaseArchive(outputs, stagedName, staged); err != nil {
		return Release{}, err
	}
	// Root-confined hard-link installation is atomic and never replaces an
	// existing archive. Root also prevents ancestor-symlink traversal escapes.
	if err := outputs.Link(stagedName, archiveName); err != nil && !errors.Is(err, os.ErrExist) {
		return Release{}, fmt.Errorf("install theme archive: %w", err)
	}
	installedDigest, err := releaseArchiveDigest(outputs, archiveName)
	if err != nil {
		return Release{}, err
	}

	if installedDigest != digest {
		return Release{}, fmt.Errorf("archive %q already has different bytes", archive)
	}
	release := Release{
		URL:    archiveURL,
		SHA256: digest,
	}
	record := map[string]json.RawMessage{}
	if previous, exists := versions[source.Version]; exists {
		_ = json.Unmarshal(previous, &record)
	}
	record["url"], _ = json.Marshal(release.URL)
	record["sha256"], _ = json.Marshal(release.SHA256)
	versions[source.Version], _ = json.Marshal(record)
	document["name"], _ = json.Marshal(source.Name)
	document["latest"], _ = json.Marshal(source.Version)
	document["versions"], _ = json.Marshal(versions)
	updated, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return Release{}, err
	}
	if err := writeReleaseIndex(outputs, indexName, append(updated, '\n')); err != nil {
		return Release{}, err
	}
	return release, nil
}

func archiveDigest(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("archive %q is not a regular file", path)
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func verifyReleaseArchive(path, name, version string) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer reader.Close()
	found := false
	for _, entry := range reader.File {
		file, err := entry.Open()
		if err != nil {
			return err
		}
		if entry.Name == "theme.json" {
			var metadata Theme
			err = json.NewDecoder(file).Decode(&metadata)
			if err == nil && (metadata.Name != name || metadata.Version != version) {
				err = fmt.Errorf("theme metadata changed while packaging")
			}
			found = true
		}
		// Read every entry through EOF to verify ZIP checksums.
		if err == nil {
			_, err = io.Copy(io.Discard, file)
		}
		file.Close()
		if err != nil {
			return err
		}
	}
	if !found {
		return fmt.Errorf("theme archive is missing theme.json")
	}
	return nil
}

func writeReleaseIndex(root *os.Root, path string, data []byte) error {
	if err := rejectReleaseSymlinks(root, path); err != nil {
		return err
	}
	name := filepath.Join(filepath.Dir(path), ".themes-index-"+rand.Text())
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	defer root.Remove(name)
	_, writeErr := file.Write(data)
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return root.Rename(name, path)
}
