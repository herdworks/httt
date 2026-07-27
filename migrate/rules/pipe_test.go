package rules_test

import (
	"testing"

	"github.com/herdworks/hypertemplates-go/migrate"
	"github.com/herdworks/hypertemplates-go/migrate/rules"
)

func TestPipeMigratesLegacyDeclarationsAndIsIdempotent(t *testing.T) {
	tests := []struct {
		name, source, want string
	}{
		{"typed", `<style ht-pipe="css:style#components"></style>`, `<style ht-pipe="to &#34;style#components&#34; as css"></style>`},
		{"selector", `<div data-ht-pipe="main"></div>`, `<div data-ht-pipe="to &#34;main&#34;"></div>`},
		{"invalid type preserved", `<div ht-pipe="unknown:main"></div>`, `<div ht-pipe="to &#34;main&#34; as unknown"></div>`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, themePath, resourcePath := writeThemeResource(t, test.source)
			report, err := (migrate.Migration{ThemePath: themePath, Rules: []rules.Rule{rules.Pipe{}}}).Run()
			if err != nil || report.Counters.Migrated != 1 || !report.Complete() {
				t.Fatalf("report = %#v, err = %v", report, err)
			}
			if got := readResource(t, resourcePath); got != test.want {
				t.Fatalf("resource = %q, want %q", got, test.want)
			}
			before := readResource(t, resourcePath)
			report, err = (migrate.Migration{ThemePath: themePath, Rules: []rules.Rule{rules.Pipe{}}}).Run()
			if err != nil || report.Counters.Migrated != 0 || readResource(t, resourcePath) != before {
				t.Fatalf("rerun = %#v, err = %v", report, err)
			}
		})
	}
}

func TestPipeReportsManualLegacyMultiDeclaration(t *testing.T) {
	source := `<style ht-pipe="css:style#components;js:script#components"></style>`
	_, themePath, resourcePath := writeThemeResource(t, source)
	report, err := (migrate.Migration{ThemePath: themePath, Rules: []rules.Rule{rules.Pipe{}}}).Run()
	if err != nil || report.Counters.Unresolved != 1 || readResource(t, resourcePath) != source {
		t.Fatalf("report = %#v, err = %v", report, err)
	}
}
