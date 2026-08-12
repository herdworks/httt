package rules

import (
	migrationtypes "github.com/herdworks/httt/migrate/types"
	"golang.org/x/net/html"
	"strconv"
	"strings"
)

// Attr migrates targeted singular ht-attr declarations.
type Attr struct{}

func (Attr) Selectors() []string { return []string{"[ht-attr]", "[data-ht-attr]"} }
func (Attr) Proposal(c *migrationtypes.Candidate) (*migrationtypes.Proposal, error) {
	if err := migrateAttr(c); err != nil {
		return nil, err
	}
	return &migrationtypes.Proposal{After: c.Element, Message: "migrated legacy ht-attr to ordinary attributes", AutoApply: true}, nil
}
func (Attr) Apply(c *migrationtypes.Candidate) error { return migrateAttr(c) }
func migrateAttr(c *migrationtypes.Candidate) error {
	if c == nil || c.Element == nil {
		return &migrationtypes.ManualMigrationRequiredError{Message: "legacy ht-attr candidate element is nil"}
	}
	var declarations []string
	dataAlias := false
	for _, a := range c.Element.Attr {
		if strings.EqualFold(a.Key, "ht-attr") || strings.EqualFold(a.Key, "data-ht-attr") {
			if len(declarations) == 0 {
				dataAlias = strings.HasPrefix(strings.ToLower(a.Key), "data-")
			}
			declarations = append(declarations, strings.Split(a.Val, ";")...)
		}
	}
	type replacement struct {
		target  string
		sources []string
	}
	var replacements []replacement
	for _, raw := range declarations {
		target, source, ok := strings.Cut(strings.TrimSpace(raw), ":")
		target, source = strings.TrimSpace(target), strings.TrimSpace(source)
		if !ok || target == "" || source == "" {
			return &migrationtypes.ManualMigrationRequiredError{Message: "ht-attr map expansion cannot be migrated automatically"}
		}
		if strings.Contains(source, "==") || strings.Contains(source, "!=") || strings.Contains(source, "~=") {
			return &migrationtypes.ManualMigrationRequiredError{Message: "conditional ht-attr declarations cannot be migrated automatically"}
		}
		var sources []string
		for _, s := range strings.Split(source, ",") {
			s = strings.TrimSpace(s)
			if s == "" {
				return &migrationtypes.ManualMigrationRequiredError{Message: "ht-attr contains an empty keypath"}
			}
			sources = append(sources, s)
		}
		replacements = append(replacements, replacement{target, sources})
	}
	seenTargets := map[string]bool{}
	for _, replacement := range replacements {
		target := strings.ToLower(replacement.target)
		if seenTargets[target] {
			return &migrationtypes.ManualMigrationRequiredError{Message: "multiple ht-attr declarations target the same attribute"}
		}
		seenTargets[target] = true
	}
	attrs := make([]html.Attribute, 0, len(c.Element.Attr)+len(replacements)+1)
	existing := map[string]string{}
	for _, a := range c.Element.Attr {
		if strings.EqualFold(a.Key, "ht-attr") || strings.EqualFold(a.Key, "data-ht-attr") {
			continue
		}
		attrs = append(attrs, a)
		existing[strings.ToLower(a.Key)] = a.Val
	}
	for _, r := range replacements {
		fallback := ""
		hasFallback := false
		if prior, ok := existing[strings.ToLower(r.target)]; ok {
			if strings.Contains(prior, "${") {
				return &migrationtypes.ManualMigrationRequiredError{Message: "target attribute already contains a template variable"}
			} else {
				fallback = prior
				hasFallback = true
			}
		}
		value := "${ " + strings.Join(r.sources, ", ")
		if hasFallback {
			value += ", " + strconv.Quote(fallback)
		}
		value += " }"
		found := false
		for i := range attrs {
			if strings.EqualFold(attrs[i].Key, r.target) {
				attrs[i].Val = value
				found = true
				break
			}
		}
		if !found {
			attrs = append(attrs, html.Attribute{Key: r.target, Val: value})
		}
	}
	apply := "ht-apply"
	if dataAlias {
		apply = "data-ht-apply"
	}
	if !hasApplyMarker(attrs) {
		attrs = append(attrs, html.Attribute{Key: apply})
	}
	c.Element.Attr = attrs
	return nil
}

// Attrs reports legacy plural syntax without modifying it.
type Attrs struct{}

func (Attrs) Selectors() []string {
	return []string{`[ht-attrs]:not([ht-attrs^="${"])`, `[data-ht-attrs]:not([data-ht-attrs^="${"])`}
}
func (Attrs) Proposal(c *migrationtypes.Candidate) (*migrationtypes.Proposal, error) {
	return nil, &migrationtypes.ManualMigrationRequiredError{Message: "legacy ht-attrs must be manually converted to a map-valued ${ ... } expression"}
}
func (Attrs) Apply(*migrationtypes.Candidate) error {
	return &migrationtypes.ManualMigrationRequiredError{Message: "legacy ht-attrs requires manual migration"}
}
