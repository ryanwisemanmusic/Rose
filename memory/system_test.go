package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSystemMemoryMissing(t *testing.T) {
	mem, err := LoadSystemMemory(t.TempDir())
	if err != nil {
		t.Fatalf("LoadSystemMemory: %v", err)
	}
	if mem != "" {
		t.Fatalf("memory = %q, want empty", mem)
	}
}

func TestLoadSystemMemoryTrimsFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, filepath.FromSlash(SystemMemoryRelPath))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("\n  useful memory  \n"), 0644); err != nil {
		t.Fatalf("write memory: %v", err)
	}

	mem, err := LoadSystemMemory(root)
	if err != nil {
		t.Fatalf("LoadSystemMemory: %v", err)
	}
	if mem != "useful memory" {
		t.Fatalf("memory = %q, want trimmed content", mem)
	}
}

func TestSystemMemoryLineFromMakeRunRequest(t *testing.T) {
	line, ok := SystemMemoryLineFromRequest("update the system.txt file with regards to 'make run' always being generated")
	if !ok {
		t.Fatal("expected request to be recognized")
	}
	want := "- When generating Makefiles, include a default build target for plain `make` and a `run` target that executes the compiled program."
	if line != want {
		t.Fatalf("line = %q, want %q", line, want)
	}
}

func TestSystemMemoryLineFromDriveDirectoryRequest(t *testing.T) {
	line, ok := SystemMemoryLineFromRequest("update system.txt: don't create a new directory called Drive")
	if !ok {
		t.Fatal("expected request to be recognized")
	}
	if !strings.Contains(line, "absolute workspace path") || !strings.Contains(line, "current workspace root") {
		t.Fatalf("line is not meaningful enough: %q", line)
	}
}

func TestSystemMemoryRejectsOneWordQuotedMemory(t *testing.T) {
	if line, ok := SystemMemoryLineFromRequest("update system.txt with 'Drive'"); ok {
		t.Fatalf("expected one-word memory to be rejected, got %q", line)
	}
}

func TestSystemMemoryLineFromSandboxTestRequest(t *testing.T) {
	line, ok := SystemMemoryLineFromRequest("update memory so sandbox_test files are created in the right place")
	if !ok {
		t.Fatal("expected request to be recognized")
	}
	if !strings.Contains(line, "named workspace subdirectory") || !strings.Contains(line, "project-relative subdirectory") {
		t.Fatalf("line is not specific enough: %q", line)
	}
}

func TestAppendSystemMemoryAvoidsDuplicate(t *testing.T) {
	root := t.TempDir()
	line := "- Prefer small focused edits."

	if _, err := AppendSystemMemory(root, line); err != nil {
		t.Fatalf("AppendSystemMemory: %v", err)
	}
	if _, err := AppendSystemMemory(root, line); err != nil {
		t.Fatalf("AppendSystemMemory duplicate: %v", err)
	}

	path := filepath.Join(root, filepath.FromSlash(SystemMemoryRelPath))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read memory: %v", err)
	}
	if got := string(data); countSubstring(got, line) != 1 {
		t.Fatalf("memory contains %q %d times:\n%s", line, countSubstring(got, line), got)
	}
}

func countSubstring(s, substr string) int {
	count := 0
	for {
		idx := strings.Index(s, substr)
		if idx == -1 {
			return count
		}
		count++
		s = s[idx+len(substr):]
	}
}
