package cli

import (
	"errors"

	"github.com/herdworks/clic"
	"github.com/herdworks/httt"
	"github.com/herdworks/httt/migrate"
)

func MigrateCommand() clic.Command {
	config := GlobalConfigOption()
	output := MigrateOutputOption()
	interactive := MigrateInteractiveOption()
	return clic.Command{
		Meta: clic.CommandMeta{
			Description: "Migrate the configured theme to current directive syntax.",
			Usage: []string{
				"{{ .Command }} -c site.yaml [--output directory] [--interactive]",
			},
		},
		Spec: clic.CommandSpec{
			Key:  "migrate",
			Help: clic.DefaultHelp,
			Options: clic.Options{
				config.Spec.Key:      &config,
				output.Spec.Key:      &output,
				interactive.Spec.Key: &interactive,
			},
			Validator: expectedArguments(0),
			Handler:   migrateHandler,
		},
	}
}

func MigrateOutputOption() clic.Option {
	return clic.Option{
		Meta: clic.OptionMeta{
			Name:        "output",
			Description: "Write a complete migrated theme to an empty output directory.",
		},
		Spec: clic.OptionSpec{
			Key:  "output",
			Kind: clic.StringOption,
		},
	}
}

func MigrateInteractiveOption() clic.Option {
	return clic.Option{
		Meta: clic.OptionMeta{
			Name:        "interactive",
			Description: "Review each migration decision interactively.",
		},
		Spec: clic.OptionSpec{
			Key:  "interactive",
			Kind: clic.BoolOption,
		},
	}
}

func migrateHandler(execution *clic.Execution) error {
	config, err := httt.LoadConfig(execution.Options["config"].StringValue())
	if err != nil {
		return err
	}
	status, err := (migrate.Migration{
		ThemePath:   config.ThemePath(),
		OutputDir:   execution.Options["output"].StringValue(),
		Interactive: execution.Options["interactive"].BoolValue(),
		Input:       execution.Stdin,
		Output:      execution.Stdout,
	}).Run()
	if err == nil || errors.Is(err, migrate.ErrInterrupted) {
		if writeErr := status.Print(execution.Stdout); writeErr != nil {
			return writeErr
		}
	}
	if err != nil {
		return err
	}
	if !status.Complete() {
		return errors.New("migration incomplete")
	}
	return nil
}
