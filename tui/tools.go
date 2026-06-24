package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var toolMarkerPattern = regexp.MustCompile(`\[(read|grep|glob):\s*(.+?)\]`)

type toolRequest struct {
	Tool string
	Arg  string
}

type toolResult struct {
	Tool   string
	Arg    string
	Output string
	Error  string
}

func extractToolMarkers(content string) []toolRequest {
	matches := toolMarkerPattern.FindAllStringSubmatch(content, -1)
	var reqs []toolRequest
	seen := make(map[string]bool)
	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		tool := strings.ToLower(strings.TrimSpace(m[1]))
		arg := strings.TrimSpace(m[2])
		if arg == "" {
			continue
		}
		key := tool + ":" + arg
		if seen[key] {
			continue
		}
		seen[key] = true
		reqs = append(reqs, toolRequest{Tool: tool, Arg: arg})
	}
	return reqs
}

func executeTool(req toolRequest, projectRoot string) toolResult {
	switch req.Tool {
	case "read":
		return executeRead(req.Arg, projectRoot)
	case "grep":
		return executeGrep(req.Arg, projectRoot)
	case "glob":
		return executeGlob(req.Arg, projectRoot)
	default:
		return toolResult{Tool: req.Tool, Arg: req.Arg, Error: "unknown tool: " + req.Tool}
	}
}

func executeRead(path, projectRoot string) toolResult {
	if !filepath.IsAbs(path) {
		path = filepath.Join(projectRoot, path)
	}
	path = filepath.Clean(path)

	if !strings.HasPrefix(path, filepath.Clean(projectRoot)) {
		return toolResult{Tool: "read", Arg: path, Error: "path outside project root"}
	}

	info, err := os.Stat(path)
	if err != nil {
		return toolResult{Tool: "read", Arg: path, Error: err.Error()}
	}
	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return toolResult{Tool: "read", Arg: path, Error: err.Error()}
		}
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		return toolResult{Tool: "read", Arg: path, Output: fmt.Sprintf("Directory listing (%d entries):\n%s", len(names), strings.Join(names, "\n"))}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return toolResult{Tool: "read", Arg: path, Error: err.Error()}
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) > 500 {
		lines = lines[:500]
		data = []byte(strings.Join(lines, "\n") + "\n... (truncated at 500 lines)")
	}

	return toolResult{Tool: "read", Arg: path, Output: string(data)}
}

func executeGrep(pattern, projectRoot string) toolResult {
	cmd := exec.Command("grep", "-rn", "--include=*.go", "--include=*.py", "--include=*.js", "--include=*.ts", "--include=*.rs", "--include=*.c", "--include=*.cpp", "--include=*.h", "--include=*.hpp", "--include=*.java", "--include=*.md", "--include=*.txt", "--include=Makefile", "-e", pattern, projectRoot)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			return toolResult{Tool: "grep", Arg: pattern, Output: string(out) + string(exitErr.Stderr)}
		}
		if len(out) == 0 {
			return toolResult{Tool: "grep", Arg: pattern, Output: "(no matches)"}
		}
	}
	output := string(out)
	if output == "" {
		output = "(no matches)"
	}
	if len(output) > 4000 {
		output = output[:4000] + "\n... (truncated at 4000 chars)"
	}
	return toolResult{Tool: "grep", Arg: pattern, Output: output}
}

func executeGlob(pattern, projectRoot string) toolResult {
	matches, err := filepath.Glob(filepath.Join(projectRoot, pattern))
	if err != nil {
		return toolResult{Tool: "glob", Arg: pattern, Error: err.Error()}
	}
	if len(matches) == 0 {
		return toolResult{Tool: "glob", Arg: pattern, Output: "(no matches)"}
	}
	var relPaths []string
	for _, m := range matches {
		rel, _ := filepath.Rel(projectRoot, m)
		relPaths = append(relPaths, rel)
	}
	output := strings.Join(relPaths, "\n")
	if len(output) > 2000 {
		output = output[:2000] + "\n... (truncated at 2000 chars)"
	}
	return toolResult{Tool: "glob", Arg: pattern, Output: output}
}

func formatToolResults(results []toolResult) string {
	var b strings.Builder
	for _, r := range results {
		b.WriteString(fmt.Sprintf("[tool: %s arg: %s]\n", r.Tool, r.Arg))
		if r.Error != "" {
			b.WriteString(fmt.Sprintf("Error: %s\n", r.Error))
		} else {
			b.WriteString(r.Output)
			if !strings.HasSuffix(r.Output, "\n") {
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
