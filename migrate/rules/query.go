package rules

import (
	"strconv"
	"strings"

	migrationtypes "github.com/herdworks/hypertemplates-go/migrate/types"
	"golang.org/x/net/html"
)

// Query migrates removed ht-query declarations into templated ordinary URLs.
type Query struct{}

func (Query) Selectors() []string { return []string{"[ht-query]", "[data-ht-query]"} }

func (Query) Proposal(candidate *migrationtypes.Candidate) (*migrationtypes.Proposal, error) {
	if _, err := migrateQuery(candidate); err != nil {
		return nil, err
	}
	return &migrationtypes.Proposal{
		After:     candidate.Element,
		Message:   "migrated legacy ht-query to templated URL attributes",
		AutoApply: true,
	}, nil
}

func (Query) Apply(candidate *migrationtypes.Candidate) error {
	warn, err := migrateQuery(candidate)
	if err != nil {
		return err
	}
	if warn {
		return &migrationtypes.MigrationWarning{Message: "a template-variable URL may already render a query string or fragment"}
	}
	return nil
}

func migrateQuery(candidate *migrationtypes.Candidate) (bool, error) {
	if candidate == nil || candidate.Element == nil {
		return false, &migrationtypes.ManualMigrationRequiredError{Message: "legacy query candidate element is nil"}
	}

	var source *html.Attribute
	for index := range candidate.Element.Attr {
		attribute := &candidate.Element.Attr[index]
		if strings.EqualFold(attribute.Key, "ht-query") || strings.EqualFold(attribute.Key, "data-ht-query") {
			if source != nil {
				return false, &migrationtypes.ManualMigrationRequiredError{Message: "element contains multiple legacy query directives"}
			}
			source = attribute
		}
	}
	if source == nil {
		return false, &migrationtypes.ManualMigrationRequiredError{Message: "legacy query attribute is missing"}
	}

	type declaration struct {
		target, name string
		keys         []string
	}
	var declarations []declaration
	for _, raw := range strings.Split(source.Val, ";") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return false, &migrationtypes.ManualMigrationRequiredError{Message: "legacy query contains an empty declaration"}
		}
		target, value, found := strings.Cut(raw, "?")
		name, keysSource, foundColon := strings.Cut(value, ":")
		if !found || !foundColon || strings.TrimSpace(target) == "" || strings.TrimSpace(name) == "" {
			return false, &migrationtypes.ManualMigrationRequiredError{Message: "legacy query declaration is malformed"}
		}
		var keys []string
		for _, key := range strings.Split(keysSource, ",") {
			key = strings.TrimSpace(key)
			if key == "" {
				return false, &migrationtypes.ManualMigrationRequiredError{Message: "legacy query contains an empty template-data keypath"}
			}
			keys = append(keys, key)
		}
		declarations = append(declarations, declaration{strings.TrimSpace(target), strings.TrimSpace(name), keys})
	}

	attrs := make([]html.Attribute, 0, len(candidate.Element.Attr)+1)
	for _, attribute := range candidate.Element.Attr {
		if !strings.EqualFold(attribute.Key, source.Key) {
			attrs = append(attrs, attribute)
		}
	}
	warn := false
	migratedTargets := map[int]bool{}
	for _, declaration := range declarations {
		index := -1
		for i := range attrs {
			if strings.EqualFold(attrs[i].Key, declaration.target) {
				index = i
				break
			}
		}
		if index < 0 {
			return false, &migrationtypes.ManualMigrationRequiredError{Message: "legacy query target attribute is missing"}
		}
		var (
			url         string
			variableURL bool
			err         error
		)
		if migratedTargets[index] {
			url = appendGeneratedQueryComponent(attrs[index].Val, declaration.name, declaration.keys)
		} else {
			url, variableURL, err = appendQueryComponent(attrs[index].Val, declaration.name, declaration.keys)
		}
		if err != nil {
			return false, err
		}
		attrs[index].Val = url
		migratedTargets[index] = true
		warn = warn || variableURL
	}
	if !hasApplyMarker(attrs) {
		apply := "ht-apply"
		if strings.HasPrefix(strings.ToLower(source.Key), "data-") {
			apply = "data-ht-apply"
		}
		attrs = append(attrs, html.Attribute{Key: apply})
	}
	candidate.Element.Attr = attrs
	return warn, nil
}

func appendGeneratedQueryComponent(rawURL, name string, keys []string) string {
	expression := "${ " + strings.Join(keys, ", ") + ", " + strconv.Quote("") + " }"
	fragment := ""
	if before, after, found := strings.Cut(rawURL, "#"); found {
		rawURL, fragment = before, "#"+after
	}
	return rawURL + "&" + name + "=" + expression + fragment
}

func appendQueryComponent(rawURL, name string, keys []string) (string, bool, error) {
	expression := "${ " + strings.Join(keys, ", ") + ", " + strconv.Quote("") + " }"
	trimmed := strings.TrimSpace(rawURL)
	if isCompleteTemplateVariable(trimmed) {
		return trimmed + "?" + name + "=" + expression, true, nil
	}
	if strings.Contains(rawURL, "${") {
		return "", false, &migrationtypes.ManualMigrationRequiredError{Message: "legacy query target URL contains mixed template interpolation"}
	}
	fragment := ""
	if before, after, found := strings.Cut(rawURL, "#"); found {
		rawURL, fragment = before, "#"+after
	}
	separator := "?"
	if strings.Contains(rawURL, "?") {
		separator = "&"
	}
	return rawURL + separator + name + "=" + expression + fragment, false, nil
}

func isCompleteTemplateVariable(value string) bool {
	return strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") && strings.Count(value, "${") == 1
}
