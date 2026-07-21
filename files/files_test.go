package files

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir test file dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("ok"), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
}

func TestFilesList(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "b.txt"))
	writeTestFile(t, filepath.Join(root, "a.txt"))
	writeTestFile(t, filepath.Join(root, "nested", "c.txt"))

	got, err := List(root, false)
	if err != nil {
		t.Fatalf("List non-recursive returned error: %v", err)
	}
	want := []string{
		"a.txt",
		"b.txt",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("List non-recursive = %#v, want %#v", got, want)
	}

	got, err = List(root, true)
	if err != nil {
		t.Fatalf("List recursive returned error: %v", err)
	}
	want = []string{
		"a.txt",
		"b.txt",
		"nested/c.txt",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("List recursive = %#v, want %#v", got, want)
	}
}

func TestFilesListMissingDirectory(t *testing.T) {
	got, err := List(filepath.Join(t.TempDir(), "missing"), true)
	if err != nil {
		t.Fatalf("List missing directory returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("List missing directory = %#v, want empty", got)
	}
}

func TestResourceDirectoryName(t *testing.T) {
	tests := []struct {
		name      string
		directory string
		fallback  string
		want      string
	}{
		{name: "default", directory: "", fallback: "static", want: "static"},
		{name: "configured", directory: "assets", fallback: "static", want: "assets"},
		{name: "nested", directory: "assets/images", fallback: "static", want: "assets/images"},
		{name: "absolute", directory: "/assets", fallback: "static", want: "/assets"},
		{name: "parent traversal", directory: "../assets", fallback: "static", want: "../assets"},
		{name: "backslash", directory: `assets\images`, fallback: "static", want: `assets\images`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ResourceDirectoryName(test.directory, test.fallback)
			if got != test.want {
				t.Fatalf("ResourceDirectoryName(%q, %q) = %q, want %q", test.directory, test.fallback, got, test.want)
			}
		})
	}
}

func TestFilesListByPattern(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "keep.md"))
	writeTestFile(t, filepath.Join(root, "skip.md"))
	writeTestFile(t, filepath.Join(root, "asset.jpg"))
	writeTestFile(t, filepath.Join(root, "nested", "keep.md"))

	got, err := ListByPattern(root, false, `^keep\.md$`, `skip`)
	if err != nil {
		t.Fatalf("ListByPattern returned error: %v", err)
	}
	want := []string{"keep.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListByPattern = %#v, want %#v", got, want)
	}

	got, err = ListByPattern(root, true, `^nested/.*\.md$`, "")
	if err != nil {
		t.Fatalf("ListByPattern recursive returned error: %v", err)
	}
	want = []string{"nested/keep.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListByPattern recursive = %#v, want %#v", got, want)
	}
}

func TestFilesListByPatternInvalidRegex(t *testing.T) {
	if _, err := ListByPattern(t.TempDir(), false, `(`, ""); err == nil {
		t.Fatal("ListByPattern invalid include regex returned nil error")
	}
	if _, err := ListByPattern(t.TempDir(), false, "", `(`); err == nil {
		t.Fatal("ListByPattern invalid exclude regex returned nil error")
	}
}

func TestFilesRelativePath(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "nested", "file.txt")
	writeTestFile(t, file)

	got, err := RelativePath(root, file)
	if err != nil {
		t.Fatalf("RelativePath returned error: %v", err)
	}
	want := "nested/file.txt"
	if got != want {
		t.Fatalf("RelativePath = %q, want %q", got, want)
	}
}

func TestFilesRelativePathRejectsOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "file.txt")
	writeTestFile(t, outside)

	if _, err := RelativePath(root, outside); err == nil {
		t.Fatal("RelativePath outside root returned nil error")
	}
}

func TestFilesLocalPathRejectsUnsafeRelativePaths(t *testing.T) {
	root := t.TempDir()
	if _, err := LocalPath(root, "/etc/passwd"); err == nil {
		t.Fatal("LocalPath absolute path returned nil error")
	}
	if _, err := LocalPath(root, "../secrets.txt"); err == nil {
		t.Fatal("LocalPath parent traversal returned nil error")
	}
	if _, err := LocalPath(root, ""); err == nil {
		t.Fatal("LocalPath empty path returned nil error")
	}
}

func TestFilesLocalPaths(t *testing.T) {
	root := t.TempDir()
	got, err := LocalPaths(root, []string{"a.txt", "nested/b.txt"})
	if err != nil {
		t.Fatalf("LocalPaths returned error: %v", err)
	}
	want := []string{
		filepath.Join(root, "a.txt"),
		filepath.Join(root, "nested", "b.txt"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LocalPaths = %#v, want %#v", got, want)
	}
}
