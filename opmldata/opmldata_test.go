package opmldata

import (
	"os"
	"testing"
)

func TestUnmarshalTemplateDataFollowingOPML(t *testing.T) {
	src, err := os.ReadFile("testdata/following.opml")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	d, err := UnmarshalTemplateData(src)
	if err != nil {
		t.Fatalf("UnmarshalTemplateData: %v", err)
	}
	if got := d.Get("head.title"); got != "Following" {
		t.Errorf("head.title = %v, want Following", got)
	}
	if got := d.Len("body.outlines"); got != 195 {
		t.Errorf("body.outlines length = %d, want 195", got)
	}
	if _, has := d["namespace"]; has {
		t.Errorf("namespace key should not be set; got %v", d["namespace"])
	}
}

func TestUnmarshalEmptyOPML(t *testing.T) {
	d, err := UnmarshalTemplateData([]byte(`<opml version="2.0"><head></head><body></body></opml>`))
	if err != nil {
		t.Fatalf("UnmarshalTemplateData: %v", err)
	}
	if _, has := d["namespace"]; has {
		t.Errorf("namespace key should not be set; got %v", d["namespace"])
	}
}
