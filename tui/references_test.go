package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindReferenceItemsIncludesEmptyDirsAndSkipsAppleDouble(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "sandbox_test"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "hello_world.cpp"), []byte("int main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "._hello_world.cpp"), []byte("sidecar"), 0644); err != nil {
		t.Fatal(err)
	}

	items := findReferenceItems(root, "", 10)

	var sawDir, sawFile, sawSidecar bool
	for _, item := range items {
		switch item.Path {
		case "sandbox_test":
			sawDir = item.IsDir && item.Empty
		case "hello_world.cpp":
			sawFile = !item.IsDir
		case "._hello_world.cpp":
			sawSidecar = true
		}
	}

	if !sawDir {
		t.Fatalf("expected empty dir in suggestions: %#v", items)
	}
	if !sawFile {
		t.Fatalf("expected file in suggestions: %#v", items)
	}
	if sawSidecar {
		t.Fatalf("did not expect AppleDouble sidecar in suggestions: %#v", items)
	}
}

func TestActiveReferenceQuery(t *testing.T) {
	start, query, ok := activeReferenceQuery("please read @sandbox/he")
	if !ok {
		t.Fatal("expected active reference query")
	}
	if start != len("please read ") || query != "sandbox/he" {
		t.Fatalf("got start=%d query=%q", start, query)
	}
}
