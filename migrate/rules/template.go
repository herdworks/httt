package rules

import (
	"strings"

	migrationtypes "github.com/herdworks/httt/migrate/types"
	"golang.org/x/net/html"
)

// Template migrates the removed legacy ht-template loop directive to ht-each.
type Template struct{}

func (Template) Selectors() []string { return []string{"[ht-template]", "[data-ht-template]"} }

func (Template) Proposal(candidate *migrationtypes.Candidate) (*migrationtypes.Proposal, error) {
	if err := migrateTemplate(candidate); err != nil {
		return nil, err
	}
	return &migrationtypes.Proposal{After: candidate.Element, Message: "migrated legacy ht-template to ht-each", AutoApply: true}, nil
}
func (Template) Apply(candidate *migrationtypes.Candidate) error {
	return migrateTemplate(candidate)
}

func migrateTemplate(candidate *migrationtypes.Candidate) error {
	if candidate == nil || candidate.Element == nil {
		return &migrationtypes.ManualMigrationRequiredError{Message: "legacy ht-template candidate element is nil"}
	}
	var matched *html.Attribute
	for i := range candidate.Element.Attr {
		if strings.EqualFold(candidate.Element.Attr[i].Key, "ht-template") || strings.EqualFold(candidate.Element.Attr[i].Key, "data-ht-template") {
			if matched != nil {
				return &migrationtypes.ManualMigrationRequiredError{Message: "element contains multiple legacy template directives"}
			}
			matched = &candidate.Element.Attr[i]
		}
	}
	if matched == nil {
		return nil
	}
	for _, attribute := range candidate.Element.Attr {
		if strings.EqualFold(attribute.Key, "ht-each") || strings.EqualFold(attribute.Key, "data-ht-each") {
			return &migrationtypes.ManualMigrationRequiredError{Message: "legacy template element already contains an active ht-each directive"}
		}
	}
	target, sources, err := parseLegacyTemplate(matched.Val)
	if err != nil {
		return err
	}
	name := "ht-each"
	if strings.HasPrefix(strings.ToLower(matched.Key), "data-") {
		name = "data-ht-each"
	}
	attributes := make([]html.Attribute, 0, len(candidate.Element.Attr))
	for _, attribute := range candidate.Element.Attr {
		if strings.EqualFold(attribute.Key, matched.Key) {
			continue
		}
		attributes = append(attributes, attribute)
	}
	attributes = append(attributes, html.Attribute{Key: name, Val: target + " in ${ " + strings.Join(sources, ", ") + " }"})
	candidate.Element.Attr = attributes
	rewriteLegacyEachMetadata(candidate.Element)
	return nil
}

func rewriteLegacyEachMetadata(element *html.Node) {
	if element == nil {
		return
	}
	if element.Type == html.TextNode {
		element.Data = rewriteLegacyEachMetadataSource(element.Data)
	}
	if element.Type == html.ElementNode {
		for i := range element.Attr {
			key := element.Attr[i].Key
			if strings.EqualFold(key, "ht-template") || strings.EqualFold(key, "data-ht-template") || strings.EqualFold(key, "ht-each") || strings.EqualFold(key, "data-ht-each") {
				continue
			}
			element.Attr[i].Val = rewriteLegacyEachMetadataSource(element.Attr[i].Val)
		}
	}
	for child := element.FirstChild; child != nil; child = child.NextSibling {
		rewriteLegacyEachMetadata(child)
	}
}

func rewriteLegacyEachMetadataSource(source string) string {
	for start := strings.Index(source, "${"); start >= 0; {
		endOffset := strings.Index(source[start+2:], "}")
		if endOffset < 0 {
			return source
		}
		end := start + 2 + endOffset
		expression := strings.NewReplacer("ht.index", "each.index", "ht.count", "each.count", "ht.total", "each.total").Replace(source[start : end+1])
		source = source[:start] + expression + source[end+1:]
		start = strings.Index(source[end+1:], "${")
		if start >= 0 {
			start += end + 1
		}
	}
	return source
}

func parseLegacyTemplate(value string) (string, []string, error) {
	// ht-template used the legacy Parameter grammar: semicolons separated
	// declarations, while commas were fallback keys within a declaration.
	// The first declaration supplies the loop binding and source; subsequent
	// declarations were nested-data mappings.
	declarations := strings.FieldsFunc(value, func(r rune) bool { return r == ';' })
	if len(declarations) == 0 {
		return "", nil, &migrationtypes.ManualMigrationRequiredError{Message: "legacy template is empty"}
	}
	head := strings.TrimSpace(declarations[0])
	target, source, ok := strings.Cut(head, ":")
	target, source = strings.TrimSpace(target), strings.TrimSpace(source)
	if !ok || target == "" || source == "" {
		return "", nil, &migrationtypes.ManualMigrationRequiredError{Message: "legacy template must begin target:source"}
	}

	// Retain the historical migration support for comma-separated identity
	// mappings in the first declaration, while treating ordinary comma entries
	// as ordered source fallbacks.
	sources := make([]string, 0)
	for _, raw := range strings.Split(source, ",") {
		part := strings.TrimSpace(raw)
		if part == "" {
			continue
		}
		if alias, mapped, hasMapping := strings.Cut(part, ":"); hasMapping {
			if strings.TrimSpace(alias) == strings.TrimSpace(mapped) {
				continue
			}
			return "", nil, &migrationtypes.ManualMigrationRequiredError{Message: "legacy template has an unresolved context alias " + part}
		}
		sources = append(sources, part)
	}
	if len(sources) == 0 {
		return "", nil, &migrationtypes.ManualMigrationRequiredError{Message: "legacy template must include a source"}
	}

	for _, raw := range declarations[1:] {
		part := strings.TrimSpace(raw)
		alias, mapped, hasMapping := strings.Cut(part, ":")
		alias, mapped = strings.TrimSpace(alias), strings.TrimSpace(mapped)
		if !hasMapping || alias == "" || mapped == "" {
			return "", nil, &migrationtypes.ManualMigrationRequiredError{Message: "legacy template has an invalid context mapping " + part}
		}
		if alias == mapped {
			continue
		}
		return "", nil, &migrationtypes.ManualMigrationRequiredError{Message: "legacy template has an unresolved context alias " + part}
	}
	return target, sources, nil
}
