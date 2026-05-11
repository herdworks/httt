package csvdata

import (
	"encoding/csv"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/herdworks/hypertemplates-go/data"
)

func TestUnmarshalTemplateDataSet(t *testing.T) {
	src := []byte("name,email,role\nAda,ada@x,admin\nBob,bob@x,member\nCarol,carol@x,member\n")
	set, err := UnmarshalTemplateDataSet(src)
	if err != nil {
		t.Fatalf("UnmarshalTemplateDataSet: %v", err)
	}
	if len(set) != 3 {
		t.Fatalf("len = %d, want 3", len(set))
	}
	if got := set[0]["name"]; got != "Ada" {
		t.Errorf("set[0].name = %v, want Ada", got)
	}
	for i, row := range set {
		for k, v := range row {
			if _, ok := v.(string); !ok {
				t.Errorf("row %d key %q: value %v is %T, want string", i, k, v, v)
			}
		}
	}
}

func TestUnmarshalTemplateData(t *testing.T) {
	src := []byte("name,email\nAda,ada@x\nBob,bob@x\n")
	d, err := UnmarshalTemplateData(src)
	if err != nil {
		t.Fatalf("UnmarshalTemplateData: %v", err)
	}
	if got := d.Len("rows"); got != 2 {
		t.Fatalf("rows length = %d, want 2", got)
	}
	if got := d.Get("rows.1.name"); got != "Ada" {
		t.Errorf("rows.1.name = %v, want Ada", got)
	}
	if got := d.Get("rows.2.email"); got != "bob@x" {
		t.Errorf("rows.2.email = %v, want bob@x", got)
	}
}

func TestUnmarshalTemplateDataCustomRowsKey(t *testing.T) {
	src := []byte("name\nAda\n")
	d, err := UnmarshalTemplateData(src, WithRowsKey("records"))
	if err != nil {
		t.Fatalf("UnmarshalTemplateData: %v", err)
	}
	if got := d.Get("records.1.name"); got != "Ada" {
		t.Errorf("records.1.name = %v, want Ada", got)
	}
	if _, has := d["rows"]; has {
		t.Errorf("default key %q should not be present when WithRowsKey overrides it", "rows")
	}
}

func TestUnmarshalTSV(t *testing.T) {
	// Embedded comma in a field must survive when the delimiter is tab.
	src := []byte("name\tnote\nAda\tHello, world\n")
	set, err := UnmarshalTemplateDataSet(src, WithDelimiter('\t'))
	if err != nil {
		t.Fatalf("UnmarshalTemplateDataSet: %v", err)
	}
	if len(set) != 1 {
		t.Fatalf("len = %d, want 1", len(set))
	}
	if got := set[0]["note"]; got != "Hello, world" {
		t.Errorf("note = %v, want %q", got, "Hello, world")
	}
}

func TestUnmarshalEmpty(t *testing.T) {
	set, err := UnmarshalTemplateDataSet(nil)
	if err != nil {
		t.Fatalf("UnmarshalTemplateDataSet(nil): %v", err)
	}
	if len(set) != 0 {
		t.Errorf("len = %d, want 0", len(set))
	}

	d, err := UnmarshalTemplateData([]byte(""))
	if err != nil {
		t.Fatalf("UnmarshalTemplateData(empty): %v", err)
	}
	rows, ok := d["rows"].(data.TemplateDataSet)
	if !ok {
		t.Fatalf("rows = %T, want TemplateDataSet", d["rows"])
	}
	if len(rows) != 0 {
		t.Errorf("rows len = %d, want 0", len(rows))
	}
}

func TestUnmarshalHeaderOnly(t *testing.T) {
	set, err := UnmarshalTemplateDataSet([]byte("a,b,c\n"))
	if err != nil {
		t.Fatalf("UnmarshalTemplateDataSet: %v", err)
	}
	if len(set) != 0 {
		t.Errorf("len = %d, want 0", len(set))
	}
}

func TestUnmarshalEmptyHeaderCell(t *testing.T) {
	cases := map[string][]byte{
		"bare empty":      []byte(",foo,bar\nx,y,z\n"),
		"whitespace only": []byte("\"  \",foo,bar\nx,y,z\n"),
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := UnmarshalTemplateDataSet(src)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			msg := err.Error()
			if !strings.Contains(msg, "column 1") {
				t.Errorf("error %q: missing column 1 reference", msg)
			}
			if !strings.Contains(msg, "values row") {
				t.Errorf("error %q: missing 'values row' hint", msg)
			}
		})
	}
}

func TestUnmarshalDuplicateHeader(t *testing.T) {
	cases := map[string][]byte{
		"exact":        []byte("name,age,name\nAda,30,X\n"),
		"trim-equal":   []byte("name,age,name \nAda,30,X\n"),
		"both-padded":  []byte(" name,age,name\nAda,30,X\n"),
		"trim-vs-trim": []byte(" name ,age,name\nAda,30,X\n"),
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := UnmarshalTemplateDataSet(src)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			msg := err.Error()
			if !strings.Contains(msg, "duplicate header") {
				t.Errorf("error %q: missing 'duplicate header'", msg)
			}
			if !strings.Contains(msg, "columns 1 and 3") {
				t.Errorf("error %q: missing 'columns 1 and 3'", msg)
			}
		})
	}
}

func TestUnmarshalRaggedRow(t *testing.T) {
	src := []byte("a,b,c\n1,2,3\n4,5\n")
	_, err := UnmarshalTemplateDataSet(src)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "csvdata: read row:") {
		t.Errorf("error %q: missing wrap prefix", err)
	}
	var fieldErr *csv.ParseError
	if !errors.As(err, &fieldErr) {
		t.Errorf("error %v: expected wrapped *csv.ParseError", err)
	}
}

func TestUnmarshalQuotedFields(t *testing.T) {
	src := []byte("name,note\n" +
		`Ada,"Hello, world"` + "\n" +
		`Bob,"She said ""hi"""` + "\n" +
		`Carol,"line1` + "\n" + `line2"` + "\n")
	set, err := UnmarshalTemplateDataSet(src)
	if err != nil {
		t.Fatalf("UnmarshalTemplateDataSet: %v", err)
	}
	if len(set) != 3 {
		t.Fatalf("len = %d, want 3", len(set))
	}
	if got := set[0]["note"]; got != "Hello, world" {
		t.Errorf("set[0].note = %q, want %q", got, "Hello, world")
	}
	if got := set[1]["note"]; got != `She said "hi"` {
		t.Errorf("set[1].note = %q, want %q", got, `She said "hi"`)
	}
	if got := set[2]["note"]; got != "line1\nline2" {
		t.Errorf("set[2].note = %q, want %q", got, "line1\nline2")
	}
}

func TestUnmarshalCRLF(t *testing.T) {
	src, err := os.ReadFile("testdata/crlf.csv")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	set, err := UnmarshalTemplateDataSet(src)
	if err != nil {
		t.Fatalf("UnmarshalTemplateDataSet: %v", err)
	}
	if len(set) != 2 {
		t.Fatalf("len = %d, want 2", len(set))
	}
	if got := set[0]["a"]; got != "1" {
		t.Errorf("set[0].a = %q, want %q", got, "1")
	}
	if got := set[1]["b"]; got != "4" {
		t.Errorf("set[1].b = %q, want %q", got, "4")
	}
}

func TestUnmarshalBOM(t *testing.T) {
	src, err := os.ReadFile("testdata/bom.csv")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	set, err := UnmarshalTemplateDataSet(src)
	if err != nil {
		t.Fatalf("UnmarshalTemplateDataSet: %v", err)
	}
	if len(set) != 1 {
		t.Fatalf("len = %d, want 1", len(set))
	}
	row := set[0]
	if _, ok := row["name"]; !ok {
		t.Errorf("row missing %q key; keys = %v", "name", keysOf(row))
	}
	for k := range row {
		if strings.HasPrefix(k, "\ufeff") {
			t.Errorf("BOM not stripped from header key %q", k)
		}
	}
}

func TestUnmarshalSampleCSVFixture(t *testing.T) {
	src, err := os.ReadFile("testdata/sample.csv")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	set, err := UnmarshalTemplateDataSet(src)
	if err != nil {
		t.Fatalf("UnmarshalTemplateDataSet: %v", err)
	}
	if len(set) != 3 {
		t.Fatalf("len = %d, want 3", len(set))
	}
	if got := set[0]["email"]; got != "ada@example.com" {
		t.Errorf("set[0].email = %v, want ada@example.com", got)
	}
}

func TestUnmarshalSampleTSVFixture(t *testing.T) {
	src, err := os.ReadFile("testdata/sample.tsv")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	set, err := UnmarshalTemplateDataSet(src, WithDelimiter('\t'))
	if err != nil {
		t.Fatalf("UnmarshalTemplateDataSet: %v", err)
	}
	if len(set) != 3 {
		t.Fatalf("len = %d, want 3", len(set))
	}
	if got := set[2]["role"]; got != "member" {
		t.Errorf("set[2].role = %v, want member", got)
	}
}

func keysOf(m data.TemplateData) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
