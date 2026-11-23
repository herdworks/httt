package cli

import (
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/herdworks/clic"
	"github.com/herdworks/httt"
	httheme "github.com/herdworks/httt/theme"
)

func ReleaseCommand() clic.Command {
	config := GlobalConfigOption()
	theme := GlobalThemeOption()
	file := PackageOutputOption()
	file.Meta.Description = "Archive path relative to the site's static directory (default: .hypertexting/themes/<slug>-v<version>.zip)."
	return clic.Command{
		Meta: clic.CommandMeta{
			Description: "Prepare a local theme archive and release index without deploying.",
			Usage: []string{
				"{{ .Command }} -c site.yaml -t themes/default/theme.json [--file downloads/theme.zip]",
			},
		},
		Spec: clic.CommandSpec{
			Key:  "release",
			Help: clic.DefaultHelp,
			Options: clic.Options{
				config.Spec.Key: &config,
				theme.Spec.Key:  &theme,
				file.Spec.Key:   &file,
			},
			Validator: expectedArguments(0),
			Handler:   releaseHandler,
		},
	}
}

func releaseHandler(execution *clic.Execution) error {
	config, err := httt.LoadConfig(execution.Options["config"].StringValue())
	if err != nil {
		return err
	}
	path := execution.Options["theme"].StringValue()
	if !filepath.IsAbs(path) {
		path = filepath.Join(config.RootDir(), path)
	}
	// Release reads name/version directly; loadTheme's display defaults must not
	// turn missing release metadata into a published Untitled Theme v0.0.0.
	theme := &httheme.Theme{URI: &url.URL{Path: path}}
	release, err := theme.Release(config.StaticDirPath(), execution.Options["file"].StringValue())
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(execution.Stdout, "theme release prepared: %s (sha256 %s)\n", release.URL, release.SHA256)
	return err
}
