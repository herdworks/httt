package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/herdworks/clic"
	"github.com/herdworks/httt"
)

func RenderCommand() clic.Command {
	theme := GlobalThemeOption()
	data := RenderDataOption()
	binding := RenderBindingOption()
	block := RenderBlockOption()
	fragment := RenderFragmentOption()
	layout := RenderLayoutOption()
	untidy := RenderUntidyOption()
	return clic.Command{
		Meta: clic.CommandMeta{
			Description: "Render a theme layout, fragment, or block.",
			Usage: []string{
				"{{ .Command }} --theme theme.json --data page.md --binding page --layout layouts/default.html",
				"{{ .Command }} -t theme.json -d data.json --fragment fragments/example",
				"{{ .Command }} -t theme.json -d data.json -b block --block example",
			},
		},
		Spec: clic.CommandSpec{
			Key:  "render",
			Help: clic.DefaultHelp,
			Options: clic.Options{
				theme.Spec.Key:    &theme,
				data.Spec.Key:     &data,
				binding.Spec.Key:  &binding,
				block.Spec.Key:    &block,
				fragment.Spec.Key: &fragment,
				layout.Spec.Key:   &layout,
				untidy.Spec.Key:   &untidy,
			},
			Validator: renderValidator,
			Handler:   renderHandler,
		},
	}
}

func RenderDataOption() clic.Option {
	return clic.Option{
		Meta: clic.OptionMeta{
			Name:        "data",
			ShortName:   "d",
			Description: "Path to a template data file.",
			Order:       1,
		},
		Spec: clic.OptionSpec{
			Key:      "data",
			Kind:     clic.StringOption,
			Required: true,
		},
	}
}

func RenderBindingOption() clic.Option {
	return clic.Option{
		Meta: clic.OptionMeta{
			Name:        "binding",
			ShortName:   "b",
			Description: "Top-level key for the loaded template data.",
			Order:       2,
		},
		Spec: clic.OptionSpec{
			Key:  "binding",
			Kind: clic.StringOption,
		},
	}
}

func RenderBlockOption() clic.Option {
	return clic.Option{
		Meta: clic.OptionMeta{
			Name:        "block",
			Description: "Theme block reference.",
			Order:       3,
		},
		Spec: clic.OptionSpec{
			Key:  "block",
			Kind: clic.StringOption,
		},
	}
}

func RenderFragmentOption() clic.Option {
	return clic.Option{
		Meta: clic.OptionMeta{
			Name:        "fragment",
			Description: "Theme fragment reference.",
			Order:       4,
		},
		Spec: clic.OptionSpec{
			Key:  "fragment",
			Kind: clic.StringOption,
		},
	}
}

func RenderLayoutOption() clic.Option {
	return clic.Option{
		Meta: clic.OptionMeta{
			Name:        "layout",
			Description: "Theme layout reference.",
			Order:       5,
		},
		Spec: clic.OptionSpec{
			Key:  "layout",
			Kind: clic.StringOption,
		},
	}
}

func RenderUntidyOption() clic.Option {
	return clic.Option{
		Meta: clic.OptionMeta{
			Name:        "untidy",
			ShortName:   "u",
			EnvName:     "{{ .EnvPrefix }}UNTIDY",
			Description: "Disable tidy mode.",
			Order:       10,
		},
		Spec: clic.OptionSpec{
			Key:  "untidy",
			Kind: clic.BoolOption,
		},
	}
}

func renderValidator(execution *clic.Execution) error {
	if err := expectedArguments(0)(execution); err != nil {
		return err
	}
	layout := execution.Options["layout"].StringValue()
	fragment := execution.Options["fragment"].StringValue()
	block := execution.Options["block"].StringValue()
	resourceRefs := 0
	for _, ref := range []string{layout, fragment, block} {
		if ref != "" {
			resourceRefs++
		}
	}
	if resourceRefs != 1 {
		return errors.New("render requires exactly one of --layout, --fragment, or --block")
	}
	binding := execution.Options["binding"].StringValue()
	if binding != "" && (binding == "theme" || strings.ContainsAny(binding, ".()")) {
		return fmt.Errorf("invalid render binding %q", binding)
	}
	return nil
}

func renderHandler(execution *clic.Execution) error {
	binding := execution.Options["binding"].StringValue()
	var output []byte
	var err error
	if block := execution.Options["block"].StringValue(); block != "" {
		output, err = httt.RenderThemeBlock(
			execution.Options["data"].StringValue(),
			execution.Options["theme"].StringValue(),
			block,
			binding,
			execution.Options["untidy"].BoolValue(),
			execution.Stderr,
		)
	} else {
		output, err = httt.RenderTheme(
			execution.Options["data"].StringValue(),
			execution.Options["theme"].StringValue(),
			execution.Options["layout"].StringValue(),
			execution.Options["fragment"].StringValue(),
			binding,
			execution.Options["untidy"].BoolValue(),
			execution.Stderr,
		)
	}
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(execution.Stdout, string(output))
	return err
}
