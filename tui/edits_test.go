package tui

import "testing"

func TestInferFileEditsFromRequestedFolder(t *testing.T) {
	prompt := "In the sandbox_test folder, create a simple hello_world.cpp file with Makefile."
	response := "Here are the files:\n\n```cpp\n#include <iostream>\nint main() { std::cout << \"Hello\"; }\n```\n\n```makefile\nhello_world: hello_world.cpp\n\tg++ -o hello_world hello_world.cpp\n```"

	edits := inferFileEdits(extractCodeBlocks(response), prompt, response)

	if len(edits) != 2 {
		t.Fatalf("expected 2 edits, got %d: %#v", len(edits), edits)
	}
	if edits[0].Path != "sandbox_test/hello_world.cpp" {
		t.Fatalf("first edit path = %q", edits[0].Path)
	}
	if edits[1].Path != "sandbox_test/Makefile" {
		t.Fatalf("second edit path = %q", edits[1].Path)
	}
}

func TestInferFileEditsFromFencePath(t *testing.T) {
	response := "```path=sandbox_test/hello_world.cpp\nint main() { return 0; }\n```"

	edits := inferFileEdits(extractCodeBlocks(response), "", response)

	if len(edits) != 1 {
		t.Fatalf("expected 1 edit, got %d", len(edits))
	}
	if edits[0].Path != "sandbox_test/hello_world.cpp" {
		t.Fatalf("edit path = %q", edits[0].Path)
	}
}

func TestInferFileEditsFromQuotedAbsoluteFencePathWithSpaces(t *testing.T) {
	response := "```path=\"/workspace/Project Root/App/sandbox_test/hello_world.cpp\"\nint main() { return 0; }\n```"

	edits := inferFileEdits(extractCodeBlocks(response), "", response)

	if len(edits) != 1 {
		t.Fatalf("expected 1 edit, got %d", len(edits))
	}
	if edits[0].Path != "/workspace/Project Root/App/sandbox_test/hello_world.cpp" {
		t.Fatalf("edit path = %q", edits[0].Path)
	}
}

func TestRepairWorkspacePathFragment(t *testing.T) {
	root := "/workspace/Project Root/App"
	for _, tc := range []struct {
		in   string
		want string
	}{
		{"Root/App/sandbox_test/hello_world.cpp", "sandbox_test/hello_world.cpp"},
		{"App/sandbox_test/hello_world.cpp", "sandbox_test/hello_world.cpp"},
	} {
		if got := repairWorkspacePathFragment(root, tc.in); got != tc.want {
			t.Fatalf("repairWorkspacePathFragment(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSuspiciousWorkspacePathFragment(t *testing.T) {
	root := "/workspace/Project Root/App"
	if !isSuspiciousWorkspacePathFragment(root, "Root/sandbox_test/hello_world.cpp") {
		t.Fatal("expected copied workspace fragment to be suspicious")
	}
	if isSuspiciousWorkspacePathFragment(root, "sandbox_test/hello_world.cpp") {
		t.Fatal("sandbox_test should be allowed")
	}
}

func TestPromptRequiresFileEdits(t *testing.T) {
	if !promptRequiresFileEdits("make me a hello_world application in sandbox_test") {
		t.Fatal("expected prompt to require file edits")
	}
	if promptRequiresFileEdits("how do I create a hello world app?") {
		t.Fatal("how-to prompt should not require file edits")
	}
}

func TestInferPostEditRunForMakefileInFolder(t *testing.T) {
	code, lang := inferPostEditRun(
		"make me a hello_world application and then run the application after",
		[]fileEdit{{Path: "sandbox_test/Makefile"}},
	)

	if code != "cd sandbox_test && make run" {
		t.Fatalf("code = %q", code)
	}
	if lang != "bash" {
		t.Fatalf("lang = %q", lang)
	}
}

func TestInferPostEditRunRequiresRunRequest(t *testing.T) {
	code, lang := inferPostEditRun(
		"make me a hello_world application",
		[]fileEdit{{Path: "sandbox_test/Makefile"}},
	)

	if code != "" || lang != "" {
		t.Fatalf("expected no inferred run, got %q %q", code, lang)
	}
}
