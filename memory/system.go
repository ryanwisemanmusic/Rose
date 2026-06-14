package memory

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const SystemMemoryRelPath = "memory/system.txt"

var quotedMemoryPattern = regexp.MustCompile(`['"]([^'"]+)['"]`)

func SystemMemoryPath(roseRoot string) string {
	if roseRoot == "" {
		return ""
	}
	return filepath.Join(roseRoot, filepath.FromSlash(SystemMemoryRelPath))
}

func LoadSystemMemory(roseRoot string) (string, error) {
	path := SystemMemoryPath(roseRoot)
	if path == "" {
		return "", nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	return strings.TrimSpace(string(data)), nil
}

func SystemMemoryLineFromRequest(request string) (string, bool) {
	lower := strings.ToLower(request)
	if !looksLikeSystemMemoryRequest(lower) {
		return "", false
	}

	if strings.Contains(lower, "drive") && (strings.Contains(lower, "directory") || strings.Contains(lower, "folder") || strings.Contains(lower, "path")) {
		return "- Do not create project-relative directories from fragments of the absolute workspace path; edit paths must stay relative to the current workspace root unless the system explicitly permits an external tool path.", true
	}

	if strings.Contains(lower, "sandbox_test") || strings.Contains(lower, "sandbox text") || strings.Contains(lower, "sandbox_text") {
		return "- When the user asks for files in a named workspace subdirectory, write them directly under that project-relative subdirectory and never under copied fragments of the absolute workspace path.", true
	}

	if strings.Contains(lower, "makefile") || strings.Contains(lower, "make run") {
		return "- When generating Makefiles, include a default build target for plain `make` and a `run` target that executes the compiled program.", true
	}

	matches := quotedMemoryPattern.FindAllStringSubmatch(request, -1)
	var best string
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		if len(match[1]) > len(best) {
			best = match[1]
		}
	}
	if best == "" {
		return "", false
	}
	if !hasEnoughMemoryMeaning(best) {
		return "", false
	}
	return formatMemoryLine(best), true
}

func AppendSystemMemory(roseRoot, line string) (string, error) {
	line = formatMemoryLine(line)
	if line == "" {
		return "", errors.New("empty system memory")
	}
	path := SystemMemoryPath(roseRoot)
	if path == "" {
		return "", os.ErrInvalid
	}

	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	content := strings.TrimRight(string(data), "\n")
	if strings.Contains(content, line) {
		return line, nil
	}
	if content == "" {
		content = defaultSystemMemoryHeader()
	}
	content += "\n" + line + "\n"

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}
	return line, nil
}

func looksLikeSystemMemoryRequest(lower string) bool {
	if strings.Contains(lower, "system.txt") {
		return true
	}
	if strings.Contains(lower, "memory/system.txt") {
		return true
	}
	if strings.Contains(lower, "update") && (strings.Contains(lower, "memory") || strings.Contains(lower, "memories")) {
		return true
	}
	if strings.Contains(lower, "remember") && strings.Contains(lower, "rose") {
		return true
	}
	return false
}

func formatMemoryLine(line string) string {
	line = strings.TrimSpace(line)
	line = strings.Trim(line, "*")
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	if strings.HasPrefix(line, "- ") {
		return line
	}
	line = strings.TrimSuffix(line, ".")
	return "- " + line + "."
}

func hasEnoughMemoryMeaning(text string) bool {
	fields := strings.Fields(text)
	if len(fields) >= 3 {
		return true
	}
	return strings.Contains(text, "/") || strings.Contains(text, "`")
}

func defaultSystemMemoryHeader() string {
	return `# Rose System Memory

Purpose: this file stores durable, generic lessons for Rose itself. It is plain text on purpose so Rose can update memory without risking unrelated Go code.

Durable memories:`
}
