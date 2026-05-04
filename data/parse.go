package data

import (
	"encoding/json"

	"gopkg.in/yaml.v3"
)

// UnmarshalJSON parses JSON-encoded bytes into a TemplateData. It is a thin
// wrapper around encoding/json.Unmarshal that returns the typed map directly.
func UnmarshalJSON(b []byte) (TemplateData, error) {
	var d TemplateData
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, err
	}
	return d, nil
}

// UnmarshalYAML parses YAML-encoded bytes into a TemplateData. It uses
// gopkg.in/yaml.v3, which decodes maps as map[string]any natively, so no key
// normalization is required. For YAML produced by yaml.v2-based libraries
// (e.g. goldmark-meta front-matter), call StringifyKeys on the result.
func UnmarshalYAML(b []byte) (TemplateData, error) {
	var d TemplateData
	if err := yaml.Unmarshal(b, &d); err != nil {
		return nil, err
	}
	return d, nil
}
