package cli

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/herdworks/clic"
	"github.com/herdworks/httt"
	httheme "github.com/herdworks/httt/theme"
)

func InstallCommand() clic.Command {
	config := GlobalConfigOption()
	latest := InstallLatestOption()
	return clic.Command{
		Meta: clic.CommandMeta{
			Description: "Install a theme from a package archive.",
			Usage: []string{
				"{{ .Command }} [--config site.yaml] [--latest] <archive-or-url>",
			},
		},
		Spec: clic.CommandSpec{
			Key:  "install",
			Help: clic.DefaultHelp,
			Options: clic.Options{
				config.Spec.Key: &config,
				latest.Spec.Key: &latest,
			},
			Validator: expectedArguments(1),
			Handler:   installHandler,
		},
	}
}

// InstallLatestOption exposes the same install behavior to downstream CLI compositions.
func InstallLatestOption() clic.Option {
	return clic.Option{
		Meta: clic.OptionMeta{
			Name:        "latest",
			Description: "Replace latest after successfully unpacking the theme, retaining its actual version metadata.",
		},
		Spec: clic.OptionSpec{
			Key:  "latest",
			Kind: clic.BoolOption,
		},
	}
}

func installHandler(execution *clic.Execution) error {
	config, err := httt.LoadConfig(execution.Options["config"].StringValue())
	if err != nil {
		return err
	}
	archive, cleanup, err := themeArchive(execution.Context, execution.Command.State.Arguments[0])
	if err != nil {
		return err
	}
	defer cleanup()
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return fmt.Errorf("open theme package %q: %w", archive, err)
	}
	defer reader.Close()
	theme, err := readThemeFromZip(reader)
	if err != nil {
		return err
	}
	if execution.Options["latest"].BoolValue() {
		if err = installLatest(reader, config.ThemesDirPath(), theme); err != nil {
			return err
		}
	} else {
		destination := filepath.Join(config.ThemesDirPath(), theme.Name, theme.Version)
		if _, err = os.Stat(destination); err == nil {
			return fmt.Errorf("theme %q version %q is already installed at %s", theme.Name, theme.Version, destination)
		} else if !os.IsNotExist(err) {
			return err
		}
		if err = extractZip(reader, destination); err != nil {
			return fmt.Errorf("extract theme package: %w", err)
		}
	}
	_, err = fmt.Fprintf(execution.Stdout, "theme %q version %q installed successfully\n", theme.Name, theme.Version)
	return err
}

// installLatest reserves the normal version directory before unpacking. Root-relative
// operations keep replacement confined to this theme, including through symlinks.
func installLatest(reader *zip.ReadCloser, themesDir string, theme httheme.Theme) error {
	if strings.EqualFold(theme.Version, "latest") {
		return errors.New("cannot install --latest with a version that aliases latest")
	}
	if err := os.MkdirAll(themesDir, 0o755); err != nil {
		return err
	}
	root, err := os.OpenRoot(themesDir)
	if err != nil {
		return err
	}
	defer root.Close()
	if err = root.Mkdir(theme.Name, 0o755); err != nil && !os.IsExist(err) {
		return err
	}
	info, err := root.Lstat(theme.Name)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("theme install target %q must be a directory, not a symlink or file", theme.Name)
	}
	target, err := root.OpenRoot(theme.Name)
	if err != nil {
		return err
	}
	defer target.Close()
	if err = checkLatestTarget(target); err != nil {
		return err
	}
	if err = target.Mkdir(theme.Version, 0o755); err != nil {
		return fmt.Errorf("create theme version directory %q: %w", theme.Version, err)
	}
	created, err := target.Lstat(theme.Version)
	if err != nil {
		return fmt.Errorf("inspect newly created version directory (cannot safely clean up): %w", err)
	}
	version, err := target.OpenRoot(theme.Version)
	if err != nil {
		return cleanupFailedVersion(target, theme.Version, created, fmt.Errorf("open new theme version directory: %w", err))
	}
	err = extractZipFiles(reader, ".", version.MkdirAll, version.OpenFile)
	version.Close()
	if err != nil {
		return cleanupFailedVersion(target, theme.Version, created, fmt.Errorf("extract theme package: %w", err))
	}
	// Recheck before deleting: conflicts must leave the old latest untouched.
	if err = checkLatestTarget(target); err != nil {
		return err
	}
	if err = target.RemoveAll("latest"); err != nil {
		return fmt.Errorf("remove previous latest: %w", err)
	}
	if err = target.Rename(theme.Version, "latest"); err != nil {
		return fmt.Errorf("rename theme version to latest: %w", err)
	}
	return nil
}

// Only remove the directory reserved by this attempt. If its identity changed,
// leave the replacement alone and include the cleanup failure with the cause.
func cleanupFailedVersion(target *os.Root, name string, created os.FileInfo, cause error) error {
	current, err := target.Lstat(name)
	if os.IsNotExist(err) {
		return cause
	}
	if err == nil && (!current.IsDir() || !os.SameFile(created, current)) {
		err = errors.New("version directory changed; refusing to remove it")
	}
	if err == nil {
		err = target.RemoveAll(name)
	}
	if err != nil {
		return errors.Join(cause, fmt.Errorf("clean up partial theme version %q: %w", name, err))
	}
	return cause
}

func checkLatestTarget(target *os.Root) error {
	info, err := target.Lstat("latest")
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("latest install target must be a directory, not a symlink or file")
	}
	return nil
}

func themeArchive(ctx context.Context, source string) (string, func(), error) {
	uri, err := url.Parse(source)
	if err == nil && (uri.Scheme == "http" || uri.Scheme == "https") && uri.Host != "" {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, uri.String(), nil)
		if err != nil {
			return "", nil, err
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			return "", nil, err
		}
		defer response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return "", nil, fmt.Errorf("download theme package: %s", response.Status)
		}
		file, err := os.CreateTemp("", "httt-theme-*.zip")
		if err != nil {
			return "", nil, err
		}
		if _, err = io.Copy(file, response.Body); err != nil {
			file.Close()
			os.Remove(file.Name())
			return "", nil, err
		}
		if err = file.Close(); err != nil {
			os.Remove(file.Name())
			return "", nil, err
		}
		return file.Name(), func() { _ = os.Remove(file.Name()) }, nil
	}
	return source, func() {}, nil
}

func readThemeFromZip(reader *zip.ReadCloser) (httheme.Theme, error) {
	for _, file := range reader.File {
		if file.Name != "theme.json" {
			continue
		}
		content, err := file.Open()
		if err != nil {
			return httheme.Theme{}, err
		}
		data, err := io.ReadAll(content)
		content.Close()
		if err != nil {
			return httheme.Theme{}, err
		}
		var theme httheme.Theme
		if err = json.Unmarshal(data, &theme); err != nil {
			return httheme.Theme{}, fmt.Errorf("parse package theme.json: %w", err)
		}
		if theme.Name == "" || theme.Version == "" {
			return httheme.Theme{}, errors.New("package theme.json requires name and version")
		}
		if err = validatePathSegment("package theme name", theme.Name); err != nil {
			return httheme.Theme{}, err
		}
		if err = validatePathSegment("package theme version", theme.Version); err != nil {
			return httheme.Theme{}, err
		}
		return theme, nil
	}
	return httheme.Theme{}, errors.New("package does not contain theme.json")
}

func extractZip(reader *zip.ReadCloser, destination string) error {
	return extractZipFiles(reader, destination, os.MkdirAll, os.OpenFile)
}

func extractZipFiles(reader *zip.ReadCloser, destination string, mkdirAll func(string, os.FileMode) error, openFile func(string, int, os.FileMode) (*os.File, error)) error {
	for _, file := range reader.File {
		target := filepath.Join(destination, file.Name)
		relative, err := filepath.Rel(destination, target)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("invalid archive path %q", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := mkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := mkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		input, err := file.Open()
		if err != nil {
			return err
		}
		output, err := openFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		closeErr := output.Close()
		input.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}
