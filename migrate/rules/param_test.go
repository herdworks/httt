package rules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/herdworks/hypertemplates-go/migrate"
	"github.com/herdworks/hypertemplates-go/migrate/rules"
	migrationtypes "github.com/herdworks/hypertemplates-go/migrate/types"
)

type externalRule struct{}

var _ rules.Rule = externalRule{}

func (externalRule) Selectors() []string {
	return []string{"[ht-custom]"}
}

func (externalRule) Proposal(*migrationtypes.Candidate) (*migrationtypes.Proposal, error) {
	return nil, nil
}

func (externalRule) Apply(*migrationtypes.Candidate) error {
	return nil
}

func TestParamMigratesLegacyParameterElements(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "ordered keypaths and ordinary default",
			source: `<span class="discarded" ht-param="page.title, site.title" default="Untitled"><b>discarded</b></span>`,
			want:   `<div ht-apply="">${ page.title, site.title, &#34;Untitled&#34; }</div>`,
		},
		{
			name:   "data alias markdown and first default in source order",
			source: `<param data-ht-param="markdown:post.summary,post.content" default="First" data-default="Second" class="discarded">`,
			want:   `<div data-ht-apply="">${ markdown(post.summary, post.content, &#34;First&#34;) }</div>`,
		},
		{
			name:   "html mode and implicit empty fallback",
			source: `<param ht-param="html:page.body">`,
			want:   `<div ht-apply="">${ page.body, &#34;&#34; }</div>`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root, themePath, resourcePath := writeThemeResource(t, test.source)
			_ = root
			report, err := (migrate.Migration{ThemePath: themePath, Rules: []rules.Rule{rules.Param{}}}).Run()
			if err != nil {
				t.Fatal(err)
			}
			if report.Counters.Migrated != 1 || report.Counters.Unresolved != 0 || report.Counters.Failed != 0 {
				t.Fatalf("report = %#v", report)
			}
			if got := readResource(t, resourcePath); got != test.want {
				t.Fatalf("resource = %q, want %q", got, test.want)
			}
		})
	}
}

func TestParamRequiresManualMigrationForAmbiguousReplacement(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "same element directive", source: `<span ht-param="page.title" ht-debug></span>`},
		{name: "descendant directive", source: `<span ht-param="page.title"><b ht-apply></b></span>`},
		{name: "multiple parameter attributes", source: `<span ht-param="page.title" data-ht-param="site.title"></span>`},
		{name: "multiple declarations", source: `<span ht-param="page.title;site.title"></span>`},
		{name: "unsupported mode", source: `<span ht-param="json:page.data"></span>`},
		{name: "empty source with explicit fallback", source: `<param ht-param default="Fallback">`},
		{name: "empty source without fallback", source: `<param ht-param>`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, themePath, resourcePath := writeThemeResource(t, test.source)
			report, err := (migrate.Migration{ThemePath: themePath, Rules: []rules.Rule{rules.Param{}}}).Run()
			if err != nil {
				t.Fatal(err)
			}
			if report.Counters.Unresolved != 1 || report.Counters.FilesChanged != 0 || report.Complete() {
				t.Fatalf("report = %#v", report)
			}
			if got := readResource(t, resourcePath); got != test.source {
				t.Fatalf("resource changed to %q", got)
			}
		})
	}
}

func TestDefaultRulesContainOnlyParamAndDebug(t *testing.T) {
	if len(migrate.DefaultRules) != 2 {
		t.Fatalf("DefaultRules length = %d, want 2", len(migrate.DefaultRules))
	}
	selectors := []string{
		strings.Join(selectorStrings(migrate.DefaultRules[0]), ","),
		strings.Join(selectorStrings(migrate.DefaultRules[1]), ","),
	}
	if got, want := strings.Join(selectors, ";"), "[ht-param],[data-ht-param];[ht-debug],[data-ht-debug]"; got != want {
		t.Fatalf("DefaultRules selectors = %q, want %q", got, want)
	}
}

func selectorStrings(rule rules.Rule) []string {
	return rule.Selectors()
}

func writeThemeResource(t *testing.T, source string) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	themePath := filepath.Join(root, "theme.json")
	resourcePath := filepath.Join(root, "fragments", "test.html")
	if err := os.MkdirAll(filepath.Dir(resourcePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(themePath, []byte(`{}`), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resourcePath, []byte(source), 0o640); err != nil {
		t.Fatal(err)
	}
	return root, themePath, resourcePath
}

func readResource(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}
