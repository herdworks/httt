package rules

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	migrationtypes "github.com/herdworks/httt/migrate/types"
	"golang.org/x/net/html"
)

// Not migrates deprecated ht-not attributes into one or more repeated ht-if
// attributes, applying De Morgan's law to legacy any-match alternatives.
type Not struct{}

type legacyConditional struct {
	operator   string
	keys       []string
	conditions []string
}

func (Not) Selectors() []string {
	return []string{"[ht-not]", "[data-ht-not]"}
}

func (Not) Proposal(candidate *migrationtypes.Candidate) (*migrationtypes.Proposal, error) {
	changed, err := migrateNot(candidate)
	if err != nil {
		return nil, err
	}
	if !changed {
		return nil, nil
	}
	return &migrationtypes.Proposal{
		After:     candidate.Element,
		Message:   "migrated deprecated ht-not to repeated ht-if conditions",
		AutoApply: true,
	}, nil
}

func (Not) Apply(candidate *migrationtypes.Candidate) error {
	_, err := migrateNot(candidate)
	return err
}

func migrateNot(candidate *migrationtypes.Candidate) (bool, error) {
	if candidate == nil || candidate.Element == nil {
		return false, &migrationtypes.ManualMigrationRequiredError{Message: "legacy ht-not candidate element is nil"}
	}
	changed := false
	attributes := make([]html.Attribute, 0, len(candidate.Element.Attr))
	for _, attribute := range candidate.Element.Attr {
		if !conditionalAttribute(attribute.Key, "ht-not", "data-ht-not") {
			attributes = append(attributes, attribute)
			continue
		}
		branches, err := migrateNegativeConditional(attribute.Val)
		if err != nil {
			return false, fmt.Errorf("migrate %q: %w", attribute.Key, err)
		}
		name := "ht-if"
		if strings.HasPrefix(strings.ToLower(attribute.Key), "data-") {
			name = "data-ht-if"
		}
		for _, branch := range branches {
			attributes = append(attributes, html.Attribute{Key: name, Val: branch})
		}
		changed = true
	}
	if changed {
		candidate.Element.Attr = attributes
	}
	return changed, nil
}

func migrateNegativeConditional(source string) ([]string, error) {
	parameters := splitLegacySemicolons(source)
	branches := make([]string, 0, len(parameters))
	for _, parameter := range parameters {
		parsed, err := parseLegacyConditional(parameter)
		if err != nil {
			return nil, err
		}
		branches = append(branches, parsed.negativeBranches()...)
	}
	if len(branches) == 0 {
		return nil, &migrationtypes.ManualMigrationRequiredError{Message: "legacy ht-not contains no alternatives"}
	}
	return branches, nil
}

func parseLegacyConditional(source string) (legacyConditional, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return legacyConditional{}, &migrationtypes.ManualMigrationRequiredError{Message: "conditional alternative is empty"}
	}
	operator, position := legacyOperator(source)
	if operator == "" {
		if colon := strings.Index(source, ":"); colon >= 0 {
			keys := splitLegacyValues(source[:colon])
			if len(keys) == 0 {
				return legacyConditional{}, &migrationtypes.ManualMigrationRequiredError{Message: "conditional comparison has no keypath"}
			}
			for _, key := range keys {
				if key == "" || strings.Contains(key, "${") {
					return legacyConditional{}, &migrationtypes.ManualMigrationRequiredError{Message: "conditional comparison contains an invalid keypath"}
				}
			}
			conditions := splitLegacyValues(source[colon+1:])
			if len(conditions) == 0 {
				conditions = []string{""}
			}
			return legacyConditional{operator: "==", keys: keys, conditions: conditions}, nil
		}
		return legacyConditional{keys: splitLegacyValues(source)}, nil
	}
	left := strings.TrimSpace(source[:position])
	if colon := strings.Index(left, ":"); colon >= 0 {
		left = strings.TrimSpace(left[colon+1:])
	}
	keys := splitLegacyValues(left)
	if len(keys) == 0 {
		return legacyConditional{}, &migrationtypes.ManualMigrationRequiredError{Message: "conditional comparison has no keypath"}
	}
	for _, key := range keys {
		if key == "" || strings.Contains(key, "${") {
			return legacyConditional{}, &migrationtypes.ManualMigrationRequiredError{Message: "conditional comparison contains an invalid keypath"}
		}
	}
	conditionSource := strings.TrimSpace(source[position+len(operator):])
	conditions := []string{conditionSource}
	if operator != "~=" {
		conditions = splitLegacyValues(conditionSource)
		if len(conditions) == 0 {
			conditions = []string{""}
		}
	}
	return legacyConditional{operator: operator, keys: keys, conditions: conditions}, nil
}

func (condition legacyConditional) negativeBranches() []string {
	if condition.operator == "" {
		branches := make([]string, 0, len(condition.keys))
		for _, key := range condition.keys {
			branches = append(branches, "!"+wrapConditionalKeypath(key))
		}
		return branches
	}
	operator := invertLegacyOperator(condition.operator)
	branches := make([]string, 0, len(condition.keys)*len(condition.conditions))
	for _, conditionValue := range condition.conditions {
		for _, key := range condition.keys {
			branches = append(branches, wrapConditionalKeypath(key)+" "+operator+" "+quoteConditionalLiteral(conditionValue))
		}
	}
	return branches
}

func wrapConditionalKeypath(key string) string {
	return "${ " + strings.TrimSpace(key) + " }"
}

func invertLegacyOperator(operator string) string {
	switch operator {
	case "==":
		return "!="
	case "!=":
		return "=="
	case "~=":
		return "!~"
	default:
		return operator
	}
}

func quoteConditionalLiteral(value string) string {
	return strconv.Quote(strings.TrimSpace(value))
}

func splitLegacyValues(source string) []string {
	return splitLegacyDelimited(source, ',')
}

func splitLegacySemicolons(source string) []string {
	return splitLegacyDelimited(source, ';')
}

func splitLegacyDelimited(source string, delimiter byte) []string {
	values := make([]string, 0, 1)
	start := 0
	for cursor := 0; cursor < len(source); cursor++ {
		if strings.HasPrefix(source[cursor:], "${") {
			end := skipLegacyTemplate(source, cursor)
			if end == 0 {
				break
			}
			cursor = end - 1
			continue
		}
		if source[cursor] == '\'' || source[cursor] == '"' {
			end := skipLegacyQuote(source, cursor)
			if end == 0 {
				break
			}
			cursor = end - 1
			continue
		}
		if source[cursor] == delimiter {
			values = append(values, strings.TrimSpace(source[start:cursor]))
			start = cursor + 1
		}
	}
	values = append(values, strings.TrimSpace(source[start:]))
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func legacyOperator(source string) (string, int) {
	operators := []string{"==", "!=", "~="}
	for cursor := 0; cursor < len(source); {
		if strings.HasPrefix(source[cursor:], "${") {
			end := skipLegacyTemplate(source, cursor)
			if end == 0 {
				return "", -1
			}
			cursor = end
			continue
		}
		if source[cursor] == '\'' || source[cursor] == '"' {
			end := skipLegacyQuote(source, cursor)
			if end == 0 {
				return "", -1
			}
			cursor = end
			continue
		}
		for _, operator := range operators {
			if strings.HasPrefix(source[cursor:], operator) {
				return operator, cursor
			}
		}
		_, size := utf8.DecodeRuneInString(source[cursor:])
		cursor += size
	}
	return "", -1
}

func skipLegacyTemplate(source string, start int) int {
	if start+2 > len(source) || source[start:start+2] != "${" {
		return 0
	}
	quote := byte(0)
	escaped := false
	for cursor := start + 2; cursor < len(source); cursor++ {
		char := source[cursor]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if char == '\\' {
				escaped = true
				continue
			}
			if char == quote {
				quote = 0
			}
			continue
		}
		if char == '\'' || char == '"' {
			quote = char
			continue
		}
		if char == '}' {
			return cursor + 1
		}
	}
	return 0
}

func skipLegacyQuote(source string, start int) int {
	quote := source[start]
	escaped := false
	for cursor := start + 1; cursor < len(source); cursor++ {
		if escaped {
			escaped = false
			continue
		}
		if source[cursor] == '\\' {
			escaped = true
			continue
		}
		if source[cursor] == quote {
			return cursor + 1
		}
	}
	return 0
}

func conditionalAttribute(name string, names ...string) bool {
	for _, candidate := range names {
		if strings.EqualFold(name, candidate) {
			return true
		}
	}
	return false
}
