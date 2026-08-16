package cli

import (
	"errors"

	"github.com/herdworks/clic"
	"github.com/herdworks/httt/check"
)

func CheckCommand() clic.Command {
	tidy := CheckTidyOption()
	return clic.Command{
		Meta: clic.CommandMeta{
			Description: "Check rendered HTML for template artifacts.",
			Usage: []string{
				"{{ .Command }} [--tidy] <file-or-directory>",
			},
			Order: 10,
		},
		Spec: clic.CommandSpec{
			Key:  "check",
			Help: clic.DefaultHelp,
			Options: clic.Options{
				tidy.Spec.Key: &tidy,
			},
			Validator: expectedArguments(1),
			Handler:   checkHandler,
		},
	}
}

func CheckTidyOption() clic.Option {
	return clic.Option{
		Meta: clic.OptionMeta{
			Name:        "tidy",
			Description: "Also report unrendered HyperTemplates directive attributes.",
		},
		Spec: clic.OptionSpec{
			Key:  "tidy",
			Kind: clic.BoolOption,
		},
	}
}

func checkHandler(execution *clic.Execution) error {
	path := execution.Command.State.Arguments[0]
	results, err := check.Check(path, check.Options{
		Tidy:   execution.Options["tidy"].BoolValue(),
		Writer: execution.Stdout,
	})
	if err != nil {
		return err
	}
	for _, result := range results {
		if result.Err != nil || len(result.Findings) > 0 {
			return errors.New("template artifacts found")
		}
	}
	return nil
}
