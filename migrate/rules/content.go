package rules

import (
	"strconv"
	"strings"

	migrationtypes "github.com/herdworks/httt/migrate/types"
	"golang.org/x/net/html"
)

// Content migrates removed ht-content directives to ht-apply template output.
type Content struct{}

func (Content) Selectors() []string { return []string{"[ht-content]", "[data-ht-content]"} }

func (Content) Proposal(candidate *migrationtypes.Candidate) (*migrationtypes.Proposal, error) {
	if err := migrateContent(candidate); err != nil {
		return nil, err
	}
	return &migrationtypes.Proposal{
		After:     candidate.Element,
		Message:   "migrated legacy ht-content to template output",
		AutoApply: true,
	}, nil
}

func (Content) Apply(candidate *migrationtypes.Candidate) error {
	return migrateContent(candidate)
}

func migrateContent(candidate *migrationtypes.Candidate) error {
	if candidate == nil || candidate.Element == nil {
		return &migrationtypes.ManualMigrationRequiredError{Message: "legacy content candidate element is nil"}
	}

	attribute, err := contentAttribute(candidate.Element)
	if err != nil {
		return err
	}
	mode, keys, err := parseLegacyContent(attribute.Val)
	if err != nil {
		return err
	}
	fallback, err := contentFallback(candidate.Element)
	if err != nil {
		return err
	}

	parts := append(append([]string(nil), keys...), strconv.Quote(fallback))
	expression := "${ "
	if mode == "markdown" {
		expression += "markdown(" + strings.Join(parts, ", ") + ")"
	} else {
		expression += strings.Join(parts, ", ")
	}
	expression += " }"

	attributes := make([]html.Attribute, 0, len(candidate.Element.Attr)+1)
	for _, current := range candidate.Element.Attr {
		if strings.EqualFold(current.Key, attribute.Key) {
			continue
		}
		attributes = append(attributes, current)
	}
	if !hasApplyMarker(attributes) {
		applyName := "ht-apply"
		if strings.HasPrefix(strings.ToLower(attribute.Key), "data-") {
			applyName = "data-ht-apply"
		}
		attributes = append(attributes, html.Attribute{Key: applyName})
	}
	candidate.Element.Attr = attributes
	for child := candidate.Element.FirstChild; child != nil; {
		next := child.NextSibling
		candidate.Element.RemoveChild(child)
		child = next
	}
	candidate.Element.AppendChild(&html.Node{Type: html.TextNode, Data: expression})
	return nil
}

func contentAttribute(element *html.Node) (*html.Attribute, error) {
	var matched *html.Attribute
	for index := range element.Attr {
		attribute := &element.Attr[index]
		if !strings.EqualFold(attribute.Key, "ht-content") && !strings.EqualFold(attribute.Key, "data-ht-content") {
			continue
		}
		if matched != nil {
			return nil, &migrationtypes.ManualMigrationRequiredError{Message: "element contains multiple legacy content directives"}
		}
		matched = attribute
	}
	if matched == nil {
		return nil, &migrationtypes.ManualMigrationRequiredError{Message: "legacy content attribute is missing"}
	}
	return matched, nil
}

func parseLegacyContent(value string) (string, []string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil, &migrationtypes.ManualMigrationRequiredError{Message: "legacy content does not contain a template-data keypath"}
	}
	if strings.Contains(value, ";") {
		return "", nil, &migrationtypes.ManualMigrationRequiredError{Message: "legacy content contains multiple declarations"}
	}
	if strings.Contains(value, "==") || strings.Contains(value, "!=") || strings.Contains(value, "~=") {
		return "", nil, &migrationtypes.ManualMigrationRequiredError{Message: "legacy content uses a condition"}
	}

	mode := ""
	if prefix, source, found := strings.Cut(value, ":"); found {
		mode = strings.ToLower(strings.TrimSpace(prefix))
		value = strings.TrimSpace(source)
		if mode != "html" && mode != "markdown" {
			return "", nil, &migrationtypes.ManualMigrationRequiredError{Message: "legacy content uses an unsupported content mode"}
		}
	}

	keys := make([]string, 0, 1)
	for _, raw := range strings.Split(value, ",") {
		key := strings.TrimSpace(raw)
		if key == "" {
			return "", nil, &migrationtypes.ManualMigrationRequiredError{Message: "legacy content contains an empty template-data keypath"}
		}
		keys = append(keys, key)
	}
	return mode, keys, nil
}

func contentFallback(element *html.Node) (string, error) {
	var fallback strings.Builder
	for child := element.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.TextNode {
			return "", &migrationtypes.ManualMigrationRequiredError{Message: "legacy content has non-text child content"}
		}
		fallback.WriteString(child.Data)
	}
	return fallback.String(), nil
}

func hasApplyMarker(attributes []html.Attribute) bool {
	for _, attribute := range attributes {
		switch strings.ToLower(attribute.Key) {
		case "ht-apply", "data-ht-apply", "hyper-template", "data-hyper-template":
			return true
		}
	}
	return false
}
