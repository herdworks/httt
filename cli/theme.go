package cli

import "github.com/herdworks/clic"

// ThemeCommand also exposes the standalone theme package/release spelling.
// Keep the root commands for consumers mounting Command() at hyperctl theme.
func ThemeCommand() clic.Command {
	return clic.Command{
		Meta: clic.CommandMeta{
			Description: "Package and release themes.",
		},
		Spec: clic.CommandSpec{
			Key:  "theme",
			Help: clic.DefaultHelp,
		},
		Commands: clic.MustCommandsFrom(
			PackageCommand,
			ReleaseCommand,
		),
	}
}
