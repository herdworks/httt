package directives

import (
	"slices"
	"strings"
	"testing"

	htdata "github.com/herdworks/hypertemplates-go/data"
	htdocument "github.com/herdworks/hypertemplates-go/document"
	"github.com/herdworks/hypertemplates-go/variable"
	"golang.org/x/net/html"
)

type recordingConditionGetter struct {
	values map[string]any
	calls  []string
}

type conditionalStringList []string

func evaluateConditionalStatementSource(source string, data variable.Getter, funcs variable.FuncMap) (bool, error) {
	statement, err := parseConditionalStatement(source)
	if err != nil {
		return false, err
	}
	return evaluateConditionalStatement(statement, data, funcs)
}

func processConditionalExpression(source string, data htdata.TemplateData) (bool, error) {
	element := &html.Node{Type: html.ElementNode, Data: "div", Attr: []html.Attribute{{Key: "ht-if", Val: source}}}
	err := (ConditionalDirective{}).Process(&htdocument.Document{Layout: element}, element, &data, nil, nil)
	return htdocument.GetAttributeString(element, "hidden") != "true", err
}

func (getter *recordingConditionGetter) Get(key string) any {
	getter.calls = append(getter.calls, key)
	return getter.values[key]
}

func TestConditionalPresenceRequiresExplicitTypedExpression(t *testing.T) {
	getter := &recordingConditionGetter{values: map[string]any{"page.title": "Title"}}
	if _, err := evaluateConditionalStatementSource("page.title", getter, nil); err == nil {
		t.Fatal("naked presence unexpectedly succeeded")
	}
	if len(getter.calls) != 0 {
		t.Fatalf("naked presence called Get(%q), want no lookup", getter.calls)
	}

	matched, err := evaluateConditionalStatementSource("${ page.title }", getter, nil)
	if err != nil || !matched {
		t.Fatalf("typed presence = %v, %v; want true", matched, err)
	}
	if got, want := getter.calls, []string{"page.title"}; len(got) != 1 || got[0] != want[0] {
		t.Fatalf("typed presence calls = %v, want %v", got, want)
	}
}

func TestConditionalPresenceTruthinessMatchesTemplateData(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  bool
	}{
		{name: "nil", value: nil, want: false},
		{name: "false", value: false, want: false},
		{name: "zero", value: 0, want: true},
		{name: "empty string", value: "", want: false},
		{name: "empty map", value: map[string]any{}, want: false},
		{name: "empty slice", value: []any{}, want: false},
		{name: "non-empty string", value: "x", want: true},
		{name: "non-empty map", value: map[string]any{"x": 1}, want: true},
		{name: "non-empty slice", value: []any{1}, want: true},
		{name: "true", value: true, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			getter := &recordingConditionGetter{values: map[string]any{"value": test.value}}
			got, err := evaluateConditionalStatementSource("${ value }", getter, nil)
			if err != nil || got != test.want {
				t.Fatalf("presence = %v, %v; want %v", got, err, test.want)
			}
		})
	}
}

func TestConditionalPresenceUsesNilOnlyFallback(t *testing.T) {
	tests := []struct {
		name     string
		primary  any
		fallback any
		want     bool
		calls    string
	}{
		{name: "false", fallback: false, want: false, calls: "primary,fallback"},
		{name: "zero", fallback: 0, want: true, calls: "primary,fallback"},
		{name: "empty string", fallback: "", want: false, calls: "primary,fallback"},
		{name: "empty map", fallback: map[string]any{}, want: false, calls: "primary,fallback"},
		{name: "empty slice", fallback: []any{}, want: false, calls: "primary,fallback"},
		{name: "non-empty value", fallback: "value", want: true, calls: "primary,fallback"},
		{name: "false does not fall through", primary: false, fallback: "value", want: false, calls: "primary"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			getter := &recordingConditionGetter{values: map[string]any{
				"primary":  test.primary,
				"fallback": test.fallback,
			}}
			got, err := evaluateConditionalStatementSource("${ primary, fallback }", getter, nil)
			if err != nil || got != test.want {
				t.Fatalf("presence = %v, %v; want %v", got, err, test.want)
			}
			if got := strings.Join(getter.calls, ","); got != test.calls {
				t.Fatalf("lookups = %q, want %q", got, test.calls)
			}
		})
	}
}

func TestConditionalComparisonsAndPrecedence(t *testing.T) {
	getter := &recordingConditionGetter{values: map[string]any{
		"page.kind":  "Post",
		"page.path":  "/blog/post",
		"page.empty": nil,
	}}
	funcs := variable.FuncMap{
		"lower": func(value any) string { return strings.ToLower(value.(string)) },
	}
	tests := []struct {
		expression string
		want       bool
	}{
		{expression: `${ page.kind } == "Post"`, want: true},
		{expression: `${ page.kind } != "draft"`, want: true},
		{expression: `${ page.kind }=!"draft"`, want: true},
		{expression: `${ page.path } =~ "^/blog/"`, want: true},
		{expression: `${ page.path }~="^/blog/"`, want: true},
		{expression: `${ page.path } !~ "^/private/"`, want: true},
		{expression: `${ page.path }~!"^/private/"`, want: true},
		{expression: `"Post" == ${ page.kind }`, want: true},
		{expression: `"post" == "post"`, want: true},
		{expression: `"post" == "Post"`, want: false},
		{expression: `${ lower(page.kind) } == "post"`, want: true},
	}
	for _, test := range tests {
		got, err := evaluateConditionalStatementSource(test.expression, getter, funcs)
		if err != nil || got != test.want {
			t.Errorf("%q = %v, %v; want %v", test.expression, got, err, test.want)
		}
	}
	nakedGetter := &recordingConditionGetter{values: map[string]any{"page.kind": "Post"}}
	if got, err := evaluateConditionalStatementSource("page.kind == Post", nakedGetter, nil); err != nil || got {
		t.Fatalf("naked comparison = %v, %v; want false literal comparison", got, err)
	}
	if len(nakedGetter.calls) != 0 {
		t.Fatalf("naked comparison called Get(%v), want no lookup", nakedGetter.calls)
	}
	resolvingGetter := &recordingConditionGetter{values: map[string]any{"page.kind": "Post"}}
	if got, err := evaluateConditionalStatementSource(`${ page.kind } == "Post"`, resolvingGetter, nil); err != nil || !got {
		t.Fatalf("typed comparison = %v, %v; want true", got, err)
	}
	if len(resolvingGetter.calls) != 1 || resolvingGetter.calls[0] != "page.kind" {
		t.Fatalf("typed comparison calls = %v, want [page.kind]", resolvingGetter.calls)
	}
}

func TestConditionalMissingComparisonValuesAreEmpty(t *testing.T) {
	getter := &recordingConditionGetter{}
	tests := []struct {
		expression string
		want       bool
	}{
		{expression: `${ missing } == ""`, want: true},
		{expression: `${ missing } != "value"`, want: true},
		{expression: `${ missing } =~ "^$"`, want: true},
		{expression: `${ missing } !~ "^$"`, want: false},
		{expression: `${ missing } =~ "["`, want: false},
	}
	for _, test := range tests {
		got, err := evaluateConditionalStatementSource(test.expression, getter, nil)
		if test.expression == `${ missing } =~ "["` {
			if err == nil {
				t.Errorf("%q returned nil error for invalid reached regex", test.expression)
			}
			continue
		}
		if err != nil || got != test.want {
			t.Errorf("%q = %v, %v; want %v", test.expression, got, err, test.want)
		}
	}
	if got, err := evaluateConditionalStatementSource("${ missing } =~ ${ missing }", getter, nil); err != nil || got {
		t.Fatalf("empty regex pair = %v, %v; want false", got, err)
	}
	if got, err := evaluateConditionalStatementSource("${ missing } !~ ${ missing }", getter, nil); err != nil || !got {
		t.Fatalf("empty regex pair inversion = %v, %v; want true", got, err)
	}
	if got, err := evaluateConditionalStatementSource(`"x" =~ ""`, getter, nil); err != nil || !got {
		t.Fatalf("one-empty right regex = %v, %v; want true", got, err)
	}
	if got, err := evaluateConditionalStatementSource(`"x" !~ ""`, getter, nil); err != nil || got {
		t.Fatalf("one-empty right regex inversion = %v, %v; want false", got, err)
	}
}

func TestConditionalQuotedLiterals(t *testing.T) {
	getter := &recordingConditionGetter{values: map[string]any{"page.title": `a "quoted" title`}}
	for _, expression := range []string{
		`${ page.title } == "a \"quoted\" title"`,
		`"contains OR and ==" == "contains OR and =="`,
		`'it\'s fine' == "it's fine"`,
	} {
		got, err := evaluateConditionalStatementSource(expression, getter, nil)
		if err != nil || !got {
			t.Errorf("%q = %v, %v; want true", expression, got, err)
		}
	}
}

func TestParseConditionalStatement(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   conditionalStatement
	}{
		{
			name:   "presence",
			source: "${ page.title }",
			want:   conditionalStatement{source: "${ page.title }", rhs: "${ page.title }"},
		},
		{
			name:   "negated presence",
			source: "!${ page.draft }",
			want:   conditionalStatement{source: "!${ page.draft }", operator: conditionalNot, rhs: "${ page.draft }"},
		},
		{
			name:   "spaced negated presence",
			source: "! ${ page.draft }",
			want:   conditionalStatement{source: "! ${ page.draft }", operator: conditionalNot, rhs: "${ page.draft }"},
		},
		{
			name:   "comparison",
			source: `${ page.kind } == "post"`,
			want:   conditionalStatement{source: `${ page.kind } == "post"`, lhs: "${ page.kind }", operator: conditionalEquals, rhs: `"post"`},
		},
		{
			name:   "compact alias comparison",
			source: `${ page.kind }=!"draft"`,
			want:   conditionalStatement{source: `${ page.kind }=!"draft"`, lhs: "${ page.kind }", operator: conditionalNotEquals, rhs: `"draft"`},
		},
		{
			name:   "not in",
			source: `"draft" not in ${ page.tags }`,
			want:   conditionalStatement{source: `"draft" not in ${ page.tags }`, lhs: `"draft"`, operator: conditionalNotIn, rhs: "${ page.tags }"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseConditionalStatement(test.source)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("parseConditionalStatement(%q) = %#v, want %#v", test.source, got, test.want)
			}
		})
	}
}

func TestConditionalDirectiveExpressionAPI(t *testing.T) {
	directive := ConditionalDirective{}
	tokens, err := directive.ParseExpression(`${ first } OR ${ second }`)
	if err != nil {
		t.Fatal(err)
	}
	if err := directive.ValidateExpression(tokens); err != nil {
		t.Fatal(err)
	}

	invalid, err := directive.ParseExpression(`${ first } OR page.title`)
	if err != nil {
		t.Fatal(err)
	}
	if err := directive.ValidateExpression(invalid); err == nil {
		t.Fatal("ValidateExpression() accepted an invalid presence statement")
	}
}

func TestConditionalDirectiveIncrementalExpressionAPI(t *testing.T) {
	directive := ConditionalDirective{}
	remainder, tokens, err := directive.ParseNextExpression(`${ first } OR ${ second }`)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(tokens), 1; got != want || tokens[0].Raw != `${ first }` {
		t.Fatalf("first segment = %#v", tokens)
	}
	if err := directive.ValidateExpression(tokens); err != nil {
		t.Fatal(err)
	}
	_, tokens, err = directive.ParseNextExpression(remainder)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(tokens), 1; got != want || tokens[0].Raw != `${ second }` {
		t.Fatalf("second segment = %#v", tokens)
	}
}

func TestConditionalMembership(t *testing.T) {
	getter := &recordingConditionGetter{values: map[string]any{
		"page.tag":        "go",
		"page.tags":       []string{"go", "templates"},
		"page.other_tags": conditionalStringList{"draft", "go"},
		"page.any_tags":   []any{1, "go", nil},
		"page.empty":      []string{},
		"page.array":      [1]string{"go"},
		"page.map":        map[string]bool{"go": true},
		"page.scalar":     "go",
	}}

	tests := []struct {
		expression string
		want       bool
	}{
		{expression: `"go" in ${ page.tags }`, want: true},
		{expression: `${ page.tag } in ${ page.tags }`, want: true},
		{expression: `${ page.other_tags } in ${ page.tags }`, want: true},
		{expression: `${ page.any_tags } in ${ page.tags }`, want: true},
		{expression: `"missing" in ${ page.tags }`, want: false},
		{expression: `"go" not in ${ page.tags }`, want: false},
		{expression: `"missing" not in ${ page.tags }`, want: true},
		{expression: `"go" in ${ page.empty }`, want: false},
		{expression: `"go" not in ${ page.empty }`, want: true},
		{expression: `"go" in ${ page.array }`, want: false},
		{expression: `"go" in ${ page.map }`, want: false},
		{expression: `"go" in ${ page.missing }`, want: false},
		{expression: `"go" not in ${ page.missing }`, want: true},
	}
	for _, test := range tests {
		got, err := evaluateConditionalStatementSource(test.expression, getter, nil)
		if err != nil || got != test.want {
			t.Errorf("%q = %v, %v; want %v", test.expression, got, err, test.want)
		}
	}
	if _, err := evaluateConditionalStatementSource(`"go" in ${ page.scalar }`, getter, nil); err == nil {
		t.Fatal("scalar membership collection returned nil error")
	}
	if _, err := evaluateConditionalStatementSource(`"go" in page.tags`, getter, nil); err == nil {
		t.Fatal("naked membership collection returned nil error")
	}
}

func TestConditionalMembershipResolvesTheValidatedRightOperandOnce(t *testing.T) {
	getter := &recordingConditionGetter{values: map[string]any{"page.tags": []string{"go"}}}
	matched, err := evaluateConditionalStatementSource(`"go" in ${ page.tags }`, getter, nil)
	if err != nil || !matched {
		t.Fatalf("membership = %v, %v; want true", matched, err)
	}
	if got, want := strings.Join(getter.calls, ","), "page.tags"; got != want {
		t.Fatalf("membership lookups = %q, want %q", got, want)
	}

	pluginCalls := 0
	funcs := variable.FuncMap{
		"tags": func(...any) string {
			pluginCalls++
			return "go"
		},
	}
	if _, err := evaluateConditionalStatementSource(`"go" in ${ tags() }`, getter, funcs); err == nil {
		t.Fatal("membership plugin collection returned nil error")
	}
	if pluginCalls != 0 {
		t.Fatalf("membership collection plugin calls = %d, want 0", pluginCalls)
	}
}

func TestConditionalRejectsUnsupportedGroupingAndOperators(t *testing.T) {
	for _, expression := range []string{
		"(${ value })",
		"${ value } or ${ other }",
		"${ value } AND ${ other }",
		"!${ value } == post",
		`"post"in${ values }`,
		`"post" not in${ values }`,
		"${ value } ==",
	} {
		if _, err := evaluateConditionalStatementSource(expression, &recordingConditionGetter{}, nil); err == nil {
			t.Errorf("%q unexpectedly succeeded", expression)
		}
	}
}

func TestConditionalEagerSyntaxAndLazyOREvaluation(t *testing.T) {
	if got, err := processConditionalExpression(`${ ok } OR ${ missing } =~ "["`, htdata.TemplateData{"ok": true}); err != nil || !got {
		t.Fatalf("unreachable invalid regex = %v, %v; want true", got, err)
	}
	if _, err := processConditionalExpression(`${ ok } OR "unterminated`, htdata.TemplateData{"ok": true}); err == nil {
		t.Fatal("unreachable malformed branch returned nil error")
	}
	if _, err := processConditionalExpression(`${ missing } OR ${ missing } =~ "["`, htdata.TemplateData{}); err == nil {
		t.Fatal("reached invalid lazy OR branch returned nil error")
	}
	if got, err := processConditionalExpression(`${ missing } OR ${ ok }`, htdata.TemplateData{"ok": true}); err != nil || !got {
		t.Fatalf("second OR statement = %v, %v; want true", got, err)
	}
	if _, err := processConditionalExpression(`${ missing } OR (${ other })`, htdata.TemplateData{}); err == nil {
		t.Fatal("reached invalid OR statement returned nil error")
	}
	if _, err := processConditionalExpression(`${ ok } OR`, htdata.TemplateData{"ok": true}); err == nil {
		t.Fatal("unreachable trailing OR returned nil error")
	}
	if _, err := processConditionalExpression(`${ missing } OR`, htdata.TemplateData{}); err == nil {
		t.Fatal("reached trailing OR returned nil error")
	}
	if got, err := processConditionalExpression(`OR ${ ok }`, htdata.TemplateData{"ok": true}); err != nil || !got {
		t.Fatalf("leading OR = %v, %v; want true", got, err)
	}
	if _, err := processConditionalExpression(`OR OR ${ ok }`, htdata.TemplateData{"ok": true}); err == nil {
		t.Fatal("repeated OR returned nil error")
	}
	if _, err := processConditionalExpression("  ", htdata.TemplateData{}); err == nil {
		t.Fatal("empty expression returned nil error")
	}

	pluginCalls := 0
	element := &html.Node{Type: html.ElementNode, Data: "div", Attr: []html.Attribute{{Key: "ht-if", Val: `${ ok } OR ${ late() } == "value"`}}}
	document := &htdocument.Document{Layout: element, Plugins: variable.Plugins{
		"late": {Name: "late", Func: func(...any) (any, error) { pluginCalls++; return "value", nil }},
	}}
	data := htdata.TemplateData{"ok": true}
	if err := (ConditionalDirective{}).Process(document, element, &data, nil, nil); err != nil {
		t.Fatal(err)
	}
	if pluginCalls != 0 {
		t.Fatalf("unreached branch called plugin %d times", pluginCalls)
	}

	element = &html.Node{Type: html.ElementNode, Data: "div", Attr: []html.Attribute{
		{Key: "ht-if", Val: "${ missing }"},
		{Key: "ht-if", Val: `${ missing } =~ "["`},
	}}
	document = &htdocument.Document{Layout: element}
	if err := (ConditionalDirective{}).Process(document, element, &htdata.TemplateData{}, nil, nil); err != nil {
		t.Fatalf("repeated attributes: %v", err)
	}
	if got := htdocument.GetAttributeString(element, "hidden"); got != "true" {
		t.Fatalf("hidden = %q, want true", got)
	}
}

func TestConditionalHideAndTidyLifecycle(t *testing.T) {
	nonTidy := &html.Node{Type: html.ElementNode, Data: "div", Attr: []html.Attribute{
		{Key: "hidden", Val: "true"},
		{Key: "ht-if", Val: "${ present }"},
	}}
	if err := (ConditionalDirective{}).Process(&htdocument.Document{Layout: nonTidy}, nonTidy, &htdata.TemplateData{"present": true}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if got := htdocument.GetAttributeString(nonTidy, "hidden"); got != "" {
		t.Fatalf("successful non-tidy condition left hidden=%q", got)
	}

	tidy := &html.Node{Type: html.ElementNode, Data: "div", Attr: []html.Attribute{{Key: "ht-if", Val: "${ present }"}}}
	if err := (ConditionalDirective{}).Process(&htdocument.Document{Layout: tidy, TidyMode: true}, tidy, &htdata.TemplateData{"present": true}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if htdocument.GetAttributeString(tidy, "ht-if") != "" {
		t.Fatalf("successful tidy condition retained directive: %v", tidy.Attr)
	}

	rejected := &html.Node{Type: html.ElementNode, Data: "div", Attr: []html.Attribute{{Key: "ht-if", Val: "${ missing }"}}}
	if err := (ConditionalDirective{}).Process(&htdocument.Document{Layout: rejected, TidyMode: true}, rejected, &htdata.TemplateData{}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if rejected.Type != html.CommentNode {
		t.Fatalf("rejected tidy element type = %v, want comment node", rejected.Type)
	}
}

func TestConditionalMixedAliasesShortCircuitBeforeLaterPass(t *testing.T) {
	element := &html.Node{Type: html.ElementNode, Data: "div", Attr: []html.Attribute{
		{Key: "ht-if", Val: "${ first }"},
		{Key: "data-ht-if", Val: "${ second }"},
	}}
	document := &htdocument.Document{Layout: element}
	data := &htdata.TemplateData{"first": false, "second": true}
	if _, err := Process(document, data, nil, nil); err != nil {
		t.Fatal(err)
	}
	if got := htdocument.GetAttributeString(element, "hidden"); got != "true" {
		t.Fatalf("mixed-alias rejection hidden=%q, want true", got)
	}
}

func TestConditionalStrictAliasesAndTidyOwnership(t *testing.T) {
	element := &html.Node{Type: html.ElementNode, Data: "div", Attr: []html.Attribute{
		{Key: "data-ht-if", Val: "${ present }"},
		{Key: "ht-if", Val: "naked"},
	}}
	strictDocument := &htdocument.Document{Layout: element, StrictMode: true, TidyMode: true}
	if err := (ConditionalDirective{}).Process(strictDocument, element, &htdata.TemplateData{"present": true}, nil, nil); err != nil {
		t.Fatalf("strict active alias: %v", err)
	}
	if htdocument.GetAttributeString(element, "data-ht-if") != "" || htdocument.GetAttributeString(element, "ht-if") != "naked" {
		t.Fatalf("strict tidy removed the wrong attributes: %v", element.Attr)
	}
}

func TestProcessRejectsRemovedHtNot(t *testing.T) {
	for _, attribute := range []string{"ht-not", "data-ht-not"} {
		layout := &html.Node{Type: html.ElementNode, Data: "article", Attr: []html.Attribute{{Key: attribute, Val: "page.draft"}}}
		document := &htdocument.Document{Layout: layout}
		_, err := Process(document, &htdata.TemplateData{}, nil, nil)
		if err == nil || !strings.Contains(err.Error(), attribute) || !strings.Contains(err.Error(), "no longer supported") || !strings.Contains(err.Error(), "<article") {
			t.Errorf("Process(%s) error = %v; want removed-directive error with element context", attribute, err)
		}
	}
}

func TestConditionalMixedAliasesUseOriginalAttributeOrder(t *testing.T) {
	element := &html.Node{Type: html.ElementNode, Data: "div", Attr: []html.Attribute{
		{Key: "ht-if", Val: "${ first }"},
		{Key: "data-ht-if", Val: "${ second }"},
	}}
	attributes := htdocument.GetAttributes(element, (ConditionalDirective{}).Attributes(false)...)
	if got, want := []string{attributes[0].Key, attributes[1].Key}, []string{"ht-if", "data-ht-if"}; !slices.Equal(got, want) {
		t.Fatalf("attribute order = %v, want %v", got, want)
	}
	if err := (ConditionalDirective{}).Process(&htdocument.Document{Layout: element}, element, &htdata.TemplateData{"first": true, "second": false}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if got := htdocument.GetAttributeString(element, "hidden"); got != "true" {
		t.Fatalf("hidden = %q, want true", got)
	}
}

func TestConditionalErrorsIncludeAttributeAndElement(t *testing.T) {
	element := &html.Node{Type: html.ElementNode, Data: "article", Attr: []html.Attribute{{Key: "ht-if", Val: "page.title"}}}
	err := (ConditionalDirective{}).Process(&htdocument.Document{Layout: element}, element, &htdata.TemplateData{}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), `"ht-if"`) || !strings.Contains(err.Error(), "<article") || !strings.Contains(err.Error(), "ht-if directives require at least one ${ ... } template variable") {
		t.Fatalf("error = %v; want attribute and element context", err)
	}
}

func TestConditionalRejectsEmptyAttribute(t *testing.T) {
	for _, source := range []string{"", " \t\n "} {
		element := &html.Node{Type: html.ElementNode, Data: "article", Attr: []html.Attribute{{Key: "ht-if", Val: source}}}
		err := (ConditionalDirective{}).Process(&htdocument.Document{Layout: element}, element, &htdata.TemplateData{}, nil, nil)
		if err == nil || !strings.Contains(err.Error(), "non-empty conditional expression") || !strings.Contains(err.Error(), `"ht-if"`) {
			t.Fatalf("Process(%q) error = %v, want contextual empty-condition error", source, err)
		}
	}
}

func TestConditionalEvaluationErrorsIncludeAttributeAndElement(t *testing.T) {
	tests := []struct {
		name      string
		attribute string
		values    map[string]any
		want      string
	}{
		{name: "invalid regex", attribute: `${ value } =~ "["`, want: "invalid regular expression"},
		{name: "scalar membership", attribute: `"go" in ${ tags }`, values: map[string]any{"tags": "go"}, want: "must resolve to a slice"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			element := &html.Node{Type: html.ElementNode, Data: "article", Attr: []html.Attribute{{Key: "ht-if", Val: test.attribute}}}
			data := htdata.TemplateData{}
			for key, value := range test.values {
				data[key] = value
			}
			err := (ConditionalDirective{}).Process(&htdocument.Document{Layout: element}, element, &data, nil, nil)
			if err == nil || !strings.Contains(err.Error(), `"ht-if"`) || !strings.Contains(err.Error(), "<article") || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v; want directive context and %q", err, test.want)
			}
		})
	}
}

func TestTemplateDataTruthyHelperMatchesHas(t *testing.T) {
	data := htdata.TemplateData{"zero": 0, "empty": "", "false": false, "value": "x"}
	for key, value := range map[string]bool{"zero": true, "empty": false, "false": false, "value": true} {
		if got := data.Has(key); got != value {
			t.Errorf("Has(%q) = %v, want %v", key, got, value)
		}
	}
}

func TestConditionalDocumentPluginOnEitherSide(t *testing.T) {
	document := &htdocument.Document{
		Layout: &html.Node{Type: html.ElementNode, Data: "div"},
	}
	document.Plugins = variable.Plugins{"kind": {Name: "kind", Func: func(values ...any) (any, error) { return values[0], nil }}}
	getter := &recordingConditionGetter{values: map[string]any{"value": "post"}}
	got, err := evaluateConditionalStatementSource(`${ kind(value) } == "post"`, getter, document.Plugins.FuncMap())
	if err != nil || !got {
		t.Fatalf("plugin comparison = %v, %v", got, err)
	}
	got, err = evaluateConditionalStatementSource(`"post" == ${ kind(value) }`, getter, document.Plugins.FuncMap())
	if err != nil || !got {
		t.Fatalf("right-side plugin comparison = %v, %v", got, err)
	}
}
