// Package csvdata extracts template data from CSV (and TSV) documents.
//
// CSV is naturally record-shaped: a header row followed by zero or more data
// rows. csvdata exposes both shapes: UnmarshalTemplateDataSet returns a
// data.TemplateDataSet (the natural []TemplateData slice), while
// UnmarshalTemplateData wraps that slice as {"rows": TemplateDataSet{...}} so
// it satisfies the parse-package dispatcher's TemplateData (map[string]any)
// contract.
//
// All cell values are returned as strings; encoding/csv does not perform type
// inference, and neither does this package. Callers can coerce in templates
// via value-DSL plugins where needed.
//
// The first row of input is treated as a required header row. Header values
// are stored unmodified as map keys; they are validated for emptiness and
// uniqueness after strings.TrimSpace, so "name" and "name " are treated as
// duplicates even though the stored key would preserve the trailing space.
package csvdata

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/herdworks/hypertemplates-go/data"
)

// DefaultRowsKey is the top-level key under which UnmarshalTemplateData
// installs the row set when wrapping a TemplateDataSet as a TemplateData.
const DefaultRowsKey = "rows"

// utf8BOM is the byte sequence for a UTF-8 byte-order mark. Excel-exported
// CSVs include it; encoding/csv does not strip it, so without removal it ends
// up as part of the first header cell.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// Option configures CSV parsing behavior.
type Option func(*options)

type options struct {
	delimiter rune
	rowsKey   string
}

// WithDelimiter sets the field separator. Defaults to ','. Use '\t' for TSV.
func WithDelimiter(r rune) Option {
	return func(o *options) { o.delimiter = r }
}

// WithRowsKey overrides the top-level key used by UnmarshalTemplateData when
// wrapping the row set. Defaults to DefaultRowsKey ("rows"). Has no effect on
// UnmarshalTemplateDataSet.
func WithRowsKey(k string) Option {
	return func(o *options) { o.rowsKey = k }
}

// UnmarshalTemplateDataSet parses CSV bytes into a TemplateDataSet. The first
// row is treated as headers and used as map keys for subsequent rows. An empty
// or header-only input returns an empty TemplateDataSet, no error.
func UnmarshalTemplateDataSet(b []byte, opts ...Option) (data.TemplateDataSet, error) {
	return parseRows(b, applyOptions(opts))
}

// UnmarshalTemplateData parses CSV bytes and returns the row set wrapped under
// the configured rows key (default "rows"). Provided to satisfy the
// parse-package dispatcher's TemplateData (map[string]any) contract.
func UnmarshalTemplateData(b []byte, opts ...Option) (data.TemplateData, error) {
	o := applyOptions(opts)
	rows, err := parseRows(b, o)
	if err != nil {
		return nil, err
	}
	return data.TemplateData{o.rowsKey: rows}, nil
}

func applyOptions(opts []Option) options {
	o := options{delimiter: ',', rowsKey: DefaultRowsKey}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

func parseRows(b []byte, o options) (data.TemplateDataSet, error) {
	if len(b) == 0 {
		return data.TemplateDataSet{}, nil
	}
	b = bytes.TrimPrefix(b, utf8BOM)

	r := csv.NewReader(bytes.NewReader(b))
	r.Comma = o.delimiter

	headers, err := r.Read()
	if err == io.EOF {
		return data.TemplateDataSet{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("csvdata: read header: %w", err)
	}
	if err := validateHeaders(headers); err != nil {
		return nil, err
	}

	out := data.TemplateDataSet{}
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("csvdata: read row: %w", err)
		}
		td := make(data.TemplateData, len(headers))
		for i, h := range headers {
			td[h] = row[i]
		}
		out = append(out, td)
	}
	return out, nil
}

// validateHeaders enforces that the first row is suitable as headers. Headers
// are checked after strings.TrimSpace, so "name" collides with "name ". The
// original (untrimmed) header is preserved as the map key in parsed rows.
func validateHeaders(headers []string) error {
	if len(headers) == 0 {
		return errors.New("csvdata: header row is empty")
	}
	seen := make(map[string]int, len(headers))
	for i, h := range headers {
		key := strings.TrimSpace(h)
		if key == "" {
			return fmt.Errorf("csvdata: header at column %d is empty or whitespace-only (looks like a values row, not a header row)", i+1)
		}
		if prev, dup := seen[key]; dup {
			return fmt.Errorf("csvdata: duplicate header %q at columns %d and %d", key, prev+1, i+1)
		}
		seen[key] = i
	}
	return nil
}
