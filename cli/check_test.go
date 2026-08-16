package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckCommand(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "index.html"), "<p>${ page.title }</p>")
	writeCLIFile(t, filepath.Join(root, "nested", "page.html"), "<p>clean</p>")
	writeCLIFile(t, filepath.Join(root, "ignored.txt"), "${ ignored }")

	output, err := runCLI(t, "check", root)
	if err == nil || !strings.Contains(output, `file "`+filepath.Join(root, "index.html")+`" errors:`) || !strings.Contains(output, "<p>${ page.title }</p>") || !strings.Contains(output, `file "`+filepath.Join(root, "nested", "page.html")+`" ok`) {
		t.Fatalf("httt check output = %q, error = %v", output, err)
	}

	if _, err = runCLI(t, "check", filepath.Join(root, "ignored.txt")); err == nil || !strings.Contains(err.Error(), "requires an HTML file or directory") {
		t.Fatalf("non-HTML check error = %v", err)
	}
}
