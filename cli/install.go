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
	return clic.Command{
		Meta: clic.CommandMeta{
			Description: "Install a theme from a package archive.",
			Usage: []string{
				"{{ .Command }} [--config site.yaml] <archive-or-url>",
			},
		},
		Spec: clic.CommandSpec{
			Key:  "install",
			Help: clic.DefaultHelp,
			Options: clic.Options{
				config.Spec.Key: &config,
			},
			Validator: expectedArguments(1),
			Handler:   installHandler,
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
	destination := filepath.Join(config.ThemesDirPath(), theme.Name, theme.Version)
	if _, err = os.Stat(destination); err == nil {
		return fmt.Errorf("theme %q version %q is already installed at %s", theme.Name, theme.Version, destination)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err = extractZip(reader, destination); err != nil {
		return fmt.Errorf("extract theme package: %w", err)
	}
	_, err = fmt.Fprintf(execution.Stdout, "theme %q version %q installed successfully\n", theme.Name, theme.Version)
	return err
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
	for _, file := range reader.File {
		target := filepath.Join(destination, file.Name)
		relative, err := filepath.Rel(destination, target)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("invalid archive path %q", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err = os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err = os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		input, err := file.Open()
		if err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
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
