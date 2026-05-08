package scripting

import (
	"errors"
	"strings"
	"testing"
)

func TestPreScan_RejectsImportDeclaration(t *testing.T) {
	cases := []string{
		`import foo from './foo.js'`,
		`import { a } from './a.js'`,
		`import * as ns from './ns.js'`,
		"\nimport defaultExport from './x.js'\nexport default function() {}\n",
	}
	for _, src := range cases {
		t.Run(src, func(t *testing.T) {
			err := preScanForUnsupported(src, "x.js")
			if err == nil {
				t.Fatal("preScanForUnsupported import: error = nil")
			}
			var perr *PluginError
			if !errors.As(err, &perr) {
				t.Fatalf("err = %v, want *PluginError", err)
			}
			if perr.Feature != "import" {
				t.Fatalf("Feature = %q, want import", perr.Feature)
			}
			if perr.Path != "x.js" {
				t.Fatalf("Path = %q, want x.js", perr.Path)
			}
		})
	}
}

func TestPreScan_RejectsExportFromForms(t *testing.T) {
	cases := []string{
		`export { x } from './m.js'`,
		`export * from './m.js'`,
		`export * as ns from './m.js'`,
	}
	for _, src := range cases {
		t.Run(src, func(t *testing.T) {
			err := preScanForUnsupported(src, "x.js")
			if err == nil {
				t.Fatal("preScanForUnsupported export-from: error = nil")
			}
			var perr *PluginError
			if !errors.As(err, &perr) {
				t.Fatalf("err = %v, want *PluginError", err)
			}
			if perr.Feature != "import" {
				t.Fatalf("Feature = %q, want import", perr.Feature)
			}
		})
	}
}

func TestPreScan_AllowsExportDefault(t *testing.T) {
	cases := []string{
		`export default function() {}`,
		`export default (x) => x`,
		"\nconst fn = () => 1\nexport default fn\n",
		`// import x from 'y'` + "\n" + `export default function() {}`,
	}
	for _, src := range cases {
		t.Run(src, func(t *testing.T) {
			if err := preScanForUnsupported(src, "x.js"); err != nil {
				t.Fatalf("preScanForUnsupported = %v, want nil", err)
			}
		})
	}
}

func TestTransform_RoundtripsExportDefault(t *testing.T) {
	src := `export default function(input) { return input.toUpperCase() }`
	out, err := transform(src, "upper.js")
	if err != nil {
		t.Fatalf("transform error = %v", err)
	}
	if !strings.Contains(out, "exports") {
		t.Fatalf("transformed output missing exports: %q", out)
	}
}

func TestTransform_PropagatesEsbuildErrors(t *testing.T) {
	src := `export default function( {`
	_, err := transform(src, "broken.js")
	if err == nil {
		t.Fatal("transform error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "esbuild transform") {
		t.Fatalf("err = %v, want substring %q", err, "esbuild transform")
	}
}

func TestPluginCacheKey_DifferentiatesByKeysAndSource(t *testing.T) {
	a := pluginCacheKey("src1", []string{"x"})
	b := pluginCacheKey("src1", []string{"y"})
	c := pluginCacheKey("src2", []string{"x"})
	d := pluginCacheKey("src1", []string{"x"})
	if a == b || a == c {
		t.Fatal("pluginCacheKey: distinct inputs produced equal keys")
	}
	if a != d {
		t.Fatal("pluginCacheKey: equal inputs produced distinct keys")
	}
}
