package hypertemplates

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/herdworks/hyperfeed-go/types/opml"
	"golang.org/x/net/html"

	"gopkg.in/yaml.v3"
)

// TemplateData
//
// HyperTemplates attempts to be as unopinionated as possible by using
// map[string]any as the core user data structure.
type TemplateData map[string]any
type TemplateDataSet []TemplateData
type OrderedTemplateData TemplateData // for YAML marshaling purposes only

// Validate(required []string)
//
// Validate TemplateData by checking if the required keys are present.
func (data TemplateData) Validate(required []string) error {
	for _, key := range required {
		if data.Get(key) == nil {
			return fmt.Errorf("data validation error: %s is required", key)
		}
	}
	return nil
}

func (data TemplateData) IsEmpty() bool {
	return len(data) == 0
}

func (data TemplateData) Keys() (keys []string) {
	for key := range data {
		keys = append(keys, key)
	}
	return keys
}

func (data TemplateData) Namespace() (namespace string) {
	if namespace, ok := data["namespace"].(string); ok {
		return namespace
	}
	if len(data.Keys()) == 1 {
		return data.Keys()[0]
	}
	return "default"
}

func (data TemplateData) ColumnValues(field string) (values []string) {
	switch strings.ToLower(field) {
	case "namespace", "namespaces":
		return data.Keys()
	default:
		return values
	}
}

// NamespaceData()
//
// Returns flattened data for the current namespace.
// If the namespace is already flat, it returns the data as is.
func (data TemplateData) NamespaceData() any {
	flattened := slices.Contains(data.Keys(), "namespace")
	if flattened {
		return data
	}
	return data[data.Namespace()]
}

func (data TemplateData) Get(key string) (value any) {
	path := strings.Split(key, ".")
	switch path[len(path)-1] {
	case "keys":
		value = data.GetKeys(path[0 : len(path)-1])
	case "length":
		value = data.GetLength(path[0 : len(path)-1])
	default:
		value = data.GetValue(path)
	}
	return value
}

func (data TemplateData) Exists(path string) bool {
	value := data.Get(path)
	switch v := value.(type) {
	case nil:
		return false // nil values are non-existent
	case bool, int, int8, int16, int32, int64, float32, float64:
		return true // unconditionally true regardless of value
	case string:
		return len(v) > 0 // non-empty strings
	case []any:
		return len(v) > 0
	case TemplateData:
		return len(v.Keys()) > 0
	case map[string]any:
		return len(slices.Collect(maps.Keys(v))) > 0 // fallback for TemplateData
	case map[any]any:
		return len(slices.Collect(maps.Keys(v))) > 0 // backup workaround for https://github.com/go-yaml/yaml/issues/139
	default:
		return false
	}
}

func (data TemplateData) GetValue(path []string) (value any) {
	value = data // make a copy of data to avoid modifying the original
	for _, k := range path {
		switch v := value.(type) {
		case TemplateData:
			value = v[k]
		case map[string]any:
			value = v[k] // fallback for TemplateData
		case map[any]any:
			value = v[k] // backup workaround for https://github.com/go-yaml/yaml/issues/139
		default:
			value = nil // not found
		}
	}
	return value
}

func (data TemplateData) GetKeys(path []string) (keys []string) {
	value := data.GetValue(path)
	switch v := value.(type) {
	case TemplateData:
		keys = v.Keys()
	case map[string]any:
		// fallback for TemplateData
		keys = make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
	case map[any]any:
		// backup workaround for https://github.com/go-yaml/yaml/issues/139
		keys = make([]string, 0, len(v))
		for k := range v {
			k, ok := k.(string)
			if !ok {
				continue
			}
			keys = append(keys, k)
		}
	default:
		keys = []string{}
	}
	return keys
}

func (data TemplateData) GetLength(path []string) (length int) {
	value := data.GetValue(path)
	switch v := value.(type) {
	case TemplateData:
		length = len(v.Keys())
	case map[string]any:
		length = len(v) // fallback for TemplateData
	case map[any]any:
		length = len(v) // backup workaround for https://github.com/go-yaml/yaml/issues/139
	case []any:
		length = len(v)
	case []TemplateData:
		length = len(v)
	case []map[string]any:
		length = len(v)
	case []map[any]any:
		length = len(v)
	case []string:
		length = len(v)
	case []int:
		length = len(v)
	case []float32:
		length = len(v)
	case []float64:
		length = len(v)
	case string:
		length = len(v)
	default:
		length = 0
	}
	return length
}

func (data TemplateData) Inspect(path []string, format string) (bytes []byte, err error) {
	var value any = data.GetValue(path)
	switch strings.ToLower(format) {
	case "json":
		if bytes, err = json.Marshal(value); err == nil {
			return bytes, err
		}
	case "yaml", "yml":
		if bytes, err = yaml.Marshal(value); err == nil {
			return bytes, err
		}
	case "opml", "xml":
		if bytes, err = xml.Marshal(value); err == nil {
			return bytes, err
		}
	default:
		return nil, fmt.Errorf("failed to inspect data (unsupported format: %s)", format)
	}
	return nil, fmt.Errorf("failed to inspect data: %s", err)
}

func (data TemplateData) Set(namespace string, value any) {
	data[namespace] = value
}

func (data TemplateData) SetKeyValue(key string, value any) (err error) {
	path := strings.Split(key, ".")
	return data.SetPathValue(path, value)
}

func (data TemplateData) SetPathValue(path []string, value any) (err error) {
	if len(path) == 0 {
		return
	}
	key := path[0]
	subpath := path[1:]
	switch v := data[key].(type) {
	case nil:
		data[key] = wrappedValue(value, subpath)
		return nil
	case TemplateData:
		return v.SetPathValue(subpath, value)
	case map[string]any:
		return TemplateData(v).SetPathValue(subpath, value)
	case map[any]any:
		m := TemplateData{}
		for k := range v {
			m[fmt.Sprintf("%v", k)] = v[k] // convert map[any]any to map[string]any
		}
		data[key] = m
		return data[key].(TemplateData).SetPathValue(subpath, value) // RECURSION!
	default:
		if len(subpath) == 0 {
			data[key] = value
			return nil
		}
		return fmt.Errorf(
			"error: cannot set TemplateData key %s (of path %s), a value with type %T already exists",
			key, strings.Join(path, "."), v,
		)
	}
}

func (data TemplateData) Delete(key string) {
	delete(data, key)
}

func (data TemplateData) Merge(incoming TemplateData, appendMode bool, verbose bool) (merged TemplateData) {
	if verbose {
		fmt.Printf("merging template data: %v\n", data)
	}
	if data == nil || data.IsEmpty() {
		if verbose {
			fmt.Printf("merged template data: %v\n", incoming)
		}
		return incoming
	}
	merged = TemplateData{}
	maps.Copy(merged, data) // work with a copy of data
	for k, v := range incoming {
		if _, ok := merged[k]; ok {
			switch mv := merged[k].(type) {
			case TemplateData:
				if iv, ok := v.(TemplateData); ok {
					merged[k] = mv.Merge(iv, appendMode, verbose)
				} else if iv, ok := v.(map[string]any); ok {
					merged[k] = mv.Merge(TemplateData(iv), appendMode, verbose)
				} else {
					merged[k] = v
				}
			case []any:
				if iv, ok := v.([]any); ok {
					if appendMode {
						merged[k] = append(mv, iv...)
					} else {
						merged[k] = iv
					}
				} else {
					merged[k] = v
				}
			case []string:
				if iv, ok := v.([]string); ok {
					if appendMode {
						merged[k] = append(mv, iv...)
					} else {
						merged[k] = iv
					}
				} else {
					merged[k] = v
				}
			case []int:
				if iv, ok := v.([]int); ok {
					if appendMode {
						merged[k] = append(mv, iv...)
					} else {
						merged[k] = iv
					}
				} else {
					merged[k] = v
				}
			case []float32:
				if iv, ok := v.([]float32); ok {
					if appendMode {
						merged[k] = append(mv, iv...)
					} else {
						merged[k] = iv
					}
				} else {
					merged[k] = v
				}
			case []float64:
				if iv, ok := v.([]float64); ok {
					if appendMode {
						merged[k] = append(mv, iv...)
					} else {
						merged[k] = iv
					}
				} else {
					merged[k] = v
				}
			default:
				fmt.Printf("unsupported merge type: %T\n", v)
				merged[k] = v
			}
		} else {
			// field does not exist, add it
			merged[k] = v
		}
	}
	if verbose {
		fmt.Printf("merged template data: %v\n", merged)
	}
	return merged
}

// Clone()
//
// Returns a deep copy of the TemplateData. Nested maps and slices are
// recursively copied so that mutations to the clone do not affect the original.
// Returns nil if the receiver is nil.
func (data TemplateData) Clone() (cloned TemplateData) {
	if data == nil {
		return nil
	}
	cloned = TemplateData{}
	for k, v := range data {
		cloned[k] = cloneValue(v)
	}
	return cloned
}

func (data TemplateData) Clean(reserved []string) (cleaned TemplateData) {
	cleaned = TemplateData{}
	for k, v := range data {
		if slices.Contains(reserved, k) {
			continue
		}
		cleaned[k] = v
	}
	return cleaned
}

// OrderedTemplateData.MarshalYAML()
//
// This method controls how OrderedTemplateData is marshaled to YAML.
//
// NOTE: To marshal TemplateData using this Marhaler you must convert the
// TemplateData struct to an OrderedTemplateData struct.
//
// Usage:
//
//	var data: TemplateData = TemplateData{ "key": "value" }
//	yaml.Marshal(OrderedTemplateData(data))
func (data OrderedTemplateData) MarshalYAML() (any, error) {
	orderedFields := []string{"created_at", "updated_at", "layout", "path", "slug", "byline", "contributors", "title", "description", "summary", "more", "tags", "unlisted", "redirect", "attachments"}
	output := &yaml.Node{Kind: yaml.MappingNode}

	// Add known fields first in order
	for _, k := range orderedFields {
		if v, ok := data[k]; ok {
			key := &yaml.Node{Kind: yaml.ScalarNode, Value: k}
			value := &yaml.Node{}
			if err := value.Encode(v); err != nil {
				return nil, err
			}
			output.Content = append(output.Content, key, value)
		}
	}

	// Add remaining/unordered fields
	for k, v := range data {
		if slices.Contains(orderedFields, k) {
			continue // skip ordered fields
		}
		key := &yaml.Node{Kind: yaml.ScalarNode, Value: k}
		value := &yaml.Node{}
		if err := value.Encode(v); err != nil {
			return nil, err
		}
		output.Content = append(output.Content, key, value)
	}
	return output, nil
}

// workaround for https://github.com/go-yaml/yaml/issues/139 (because goldmark-meta uses yaml.v2)
// inspired by go-yaml/yaml#139 comments, stackoverflow, and github.com/gohugoio/hugo/pull/4138
func (data TemplateData) StringifyMapKeys() {
	for k, v := range data {
		data[k] = data.StringifyNestedMapKey(v)
	}
}
func (data TemplateData) StringifyNestedMapKey(incoming any) any {
	switch incoming := incoming.(type) {
	case []any:
		s := make([]any, len(incoming))
		for i, v := range incoming {
			s[i] = data.StringifyNestedMapKey(v) // RECURSION!
		}
		return s
	case map[any]any:
		m := make(map[string]any)
		for k, v := range incoming {
			m[fmt.Sprintf("%v", k)] = data.StringifyNestedMapKey(v) // RECURSION!
		}
		return m
	default:
		return incoming
	}
}

// =============================================================================
// Helper types, methods, and functions
// =============================================================================

func ParseTemplateDataFile(path string, config *HyperMarkConfig) (data TemplateData, err error) {
	var fileBytes []byte
	var fileData TemplateData
	if fileBytes, err = os.ReadFile(path); err != nil {
		return TemplateData{}, err
	}
	if len(fileBytes) == 0 {
		return TemplateData{}, err // skip empty files
	}
	if fileData, err = ParseTemplateDataBytes(fileBytes, filepath.Ext(path), config); err != nil {
		return TemplateData{}, err
	}
	return fileData, nil
}
func ParseTemplateDataBytes(bytes []byte, format string, config *HyperMarkConfig) (data TemplateData, err error) {
	switch format {
	case ".json":
		if err = json.Unmarshal(bytes, &data); err != nil {
			return nil, err
		}
	case ".yaml", ".yml":
		if err = yaml.Unmarshal(bytes, &data); err != nil {
			return nil, err
		}
	case ".opml":
		return ParseTemplateDataOPML(bytes)
	case ".md":
		return ParseTemplateDataMarkdown(bytes, config)
	case ".html":
		return ParseTemplateDataHTML(bytes)
	default:
		return nil, fmt.Errorf("unsupported file type: %s", format)
	}
	return data, nil
}
func ParseTemplateDataOPML(opmlBytes []byte) (TemplateData, error) {
	var jsonBytes []byte
	var opml *opml.OPML
	var opmlNS string
	var data TemplateData = TemplateData{}
	var err error

	// unmarshal opmlBytes to an opml.OPML struct and get omplNS
	if err = xml.Unmarshal(opmlBytes, &opml); err != nil {
		return nil, err
	}
	if opmlNS = opml.Head.Title; len(opmlNS) == 0 {
		opmlNS = "opml"
	}
	opmlNS = strings.ReplaceAll(opmlNS, " ", "")
	opmlNS = strings.ReplaceAll(opmlNS, ".", "")

	// marshal the OPML to JSON, then unmarshal JSON to TemplateData
	if jsonBytes, err = json.Marshal(opml); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(jsonBytes, &data); err != nil {
		return nil, err
	}
	data.Set("namespace", strings.ToLower(opmlNS))
	return data, nil
}
func ParseTemplateDataMarkdown(mdBytes []byte, config *HyperMarkConfig) (data TemplateData, err error) {
	md := HyperMark{}.New(config)
	if err = md.Parse(mdBytes); err != nil {
		return nil, err
	}
	return md.Data(), nil
}
func ParseTemplateDataHTML(htmlBytes []byte) (data TemplateData, err error) {
	var document *html.Node
	data = TemplateData{}
	if document, err = html.Parse(bytes.NewReader(htmlBytes)); err != nil {
		return nil, err
	}
	if title := Query(document, "title"); title != nil {
		data["title"] = title.FirstChild.Data // extract text content of title element
	}
	if description := Query(document, "meta[name=description]"); description != nil {
		data["description"] = getAttributeString(description, "content")
	}
	if createdAt := Query(document, "meta[name=created_at]"); createdAt != nil {
		data["created_at"] = getAttributeString(createdAt, "content")
	}
	if updatedAt := Query(document, "meta[name=updated_at]"); updatedAt != nil {
		data["updated_at"] = getAttributeString(updatedAt, "content")
	}
	return data, nil
}

func wrappedValue(value any, path []string) any {
	if len(path) == 0 {
		return value
	}
	for len(path) > 0 {
		key := path[len(path)-1]
		path = path[:len(path)-1]        // remove last element
		value = TemplateData{key: value} // wrap the value in TemplateData
	}
	return value
}

func cloneValue(value any) any {
	switch v := value.(type) {
	case TemplateData:
		return v.Clone()
	case map[string]any:
		m := make(map[string]any, len(v))
		for k, vv := range v {
			m[k] = cloneValue(vv)
		}
		return m
	case map[any]any:
		// workaround for https://github.com/go-yaml/yaml/issues/139
		m := make(map[any]any, len(v))
		for k, vv := range v {
			m[k] = cloneValue(vv)
		}
		return m
	case []any:
		s := make([]any, len(v))
		for i, vv := range v {
			s[i] = cloneValue(vv)
		}
		return s
	case []TemplateData:
		s := make([]TemplateData, len(v))
		for i, vv := range v {
			s[i] = vv.Clone()
		}
		return s
	case []map[string]any:
		s := make([]map[string]any, len(v))
		for i, vv := range v {
			s[i] = cloneValue(vv).(map[string]any)
		}
		return s
	case []map[any]any:
		s := make([]map[any]any, len(v))
		for i, vv := range v {
			s[i] = cloneValue(vv).(map[any]any)
		}
		return s
	case []string:
		return slices.Clone(v)
	case []int:
		return slices.Clone(v)
	case []float32:
		return slices.Clone(v)
	case []float64:
		return slices.Clone(v)
	default:
		return value
	}
}
