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
	if got := d.Get("namespace"); got != "following" {
		t.Errorf("namespace = %v, want following", got)
	}
}

func TestNamespaceFallback(t *testing.T) {
	// OPML with empty title falls back to "opml" namespace.
	d, err := UnmarshalTemplateData([]byte(`<opml version="2.0"><head></head><body></body></opml>`))
	if err != nil {
		t.Fatalf("UnmarshalTemplateData: %v", err)
	}
	if got := d.Get("namespace"); got != "opml" {
		t.Errorf("namespace = %v, want opml", got)
	}
}

func TestNamespaceNormalization(t *testing.T) {
	src := `<opml version="2.0"><head><title>My Site.com Subscriptions</title></head><body></body></opml>`
	d, err := UnmarshalTemplateData([]byte(src))
	if err != nil {
		t.Fatalf("UnmarshalTemplateData: %v", err)
	}
	if got := d.Get("namespace"); got != "mysitecomsubscriptions" {
		t.Errorf("namespace = %v, want mysitecomsubscriptions", got)
	}
}
