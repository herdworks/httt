package rules

import (
	"strconv"
	"strings"

	migrationtypes "github.com/herdworks/hypertemplates-go/migrate/types"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Param migrates legacy ht-param elements into ht-apply template output.
type Param struct{}

func (Param) Selectors() []string {
	return []string{"[ht-param]", "[data-ht-param]"}
}

func (Param) Proposal(candidate *migrationtypes.Candidate) (*migrationtypes.Proposal, error) {
	if err := applyParam(candidate); err != nil {
		return nil, err
	}
	return &migrationtypes.Proposal{
		After:     candidate.Element,
		Message:   "replaced legacy parameter element with template output",
		AutoApply: true,
	}, nil
}

func (Param) Apply(candidate *migrationtypes.Candidate) error {
	return applyParam(candidate)
}

func applyParam(candidate *migrationtypes.Candidate) error {
	replacement, err := paramReplacement(candidate)
	if err != nil {
		return err
	}
	return candidate.ReplaceElement(replacement)
}

func paramReplacement(candidate *migrationtypes.Candidate) (*html.Node, error) {
	if candidate == nil || candidate.Element == nil {
		return nil, &migrationtypes.ManualMigrationRequiredError{Message: "parameter candidate element is nil"}
	}
	var attributes []*html.Attribute
	for index := range candidate.Element.Attr {
		attribute := &candidate.Element.Attr[index]
		if strings.EqualFold(attribute.Key, "ht-param") || strings.EqualFold(attribute.Key, "data-ht-param") {
			attributes = append(attributes, attribute)
		}
	}
	if len(attributes) != 1 {
		return nil, &migrationtypes.ManualMigrationRequiredError{Message: "element contains multiple parameter directives"}
	}
	attribute := attributes[0]
	if containsOtherDirective(candidate, "ht-param", "data-ht-param") {
		return nil, &migrationtypes.ManualMigrationRequiredError{Message: "parameter subtree contains another directive"}
	}
	mode, keys, err := parseParam(attribute.Val)
	if err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, &migrationtypes.ManualMigrationRequiredError{Message: "parameter does not contain a template-data keypath"}
	}
	fallback := ""
	for _, attribute := range candidate.Element.Attr {
		if strings.EqualFold(attribute.Key, "default") || strings.EqualFold(attribute.Key, "data-default") {
			fallback = attribute.Val
			break
		}
	}
	parts := append(append([]string(nil), keys...), strconv.Quote(fallback))
	expression := "${ "
	if mode == "markdown" {
		expression += "markdown(" + strings.Join(parts, ", ") + ")"
	} else {
		expression += strings.Join(parts, ", ")
	}
	expression += " }"
	applyName := "ht-apply"
	if strings.HasPrefix(strings.ToLower(attribute.Key), "data-") {
		applyName = "data-ht-apply"
	}
	replacement := &html.Node{
		Type:     html.ElementNode,
		DataAtom: atom.Div,
		Data:     atom.Div.String(),
		Attr:     []html.Attribute{{Key: applyName}},
	}
	replacement.AppendChild(&html.Node{Type: html.TextNode, Data: expression})
	return replacement, nil
}

func parseParam(value string) (string, []string, error) {
	parameters := strings.Split(value, ";")
	var source string
	for _, parameter := range parameters {
		parameter = strings.TrimSpace(parameter)
		if parameter == "" {
			continue
		}
		if source != "" {
			return "", nil, &migrationtypes.ManualMigrationRequiredError{Message: "parameter contains multiple legacy declarations"}
		}
		source = parameter
	}
	mode := ""
	if colon := strings.Index(source, ":"); colon >= 0 {
		mode = strings.ToLower(strings.TrimSpace(source[:colon]))
		source = source[colon+1:]
	}
	if mode != "" && mode != "html" && mode != "markdown" {
		return "", nil, &migrationtypes.ManualMigrationRequiredError{Message: "parameter uses an unsupported content mode"}
	}
	if strings.Contains(source, "==") || strings.Contains(source, "!=") || strings.Contains(source, "~=") {
		return "", nil, &migrationtypes.ManualMigrationRequiredError{Message: "parameter uses a legacy condition"}
	}
	var keys []string
	for _, key := range strings.Split(source, ",") {
		key = strings.TrimSpace(key)
		if key != "" {
			keys = append(keys, key)
		}
	}
	return mode, keys, nil
}
